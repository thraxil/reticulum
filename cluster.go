package main

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"time"

	"github.com/go-kit/kit/log"
)

// represents what our Node nows about the cluster
// ie, itself and its neighbors
// TODO: we do a lot of lookups of neighbors by UUID
// there should probably be a map for that so we don't
// have to run through the whole list every time
type cluster struct {
	Myself    nodeData
	neighbors map[string]nodeData
	chF       chan func()

	recentlyVerified []imageRecord
	recentlyUploaded []imageRecord
	recentlyStashed  []imageRecord
}

func newCluster(myself nodeData) *cluster {
	c := &cluster{
		Myself:    myself,
		neighbors: make(map[string]nodeData),
		chF:       make(chan func()),
	}
	go c.backend()
	return c
}

func (c *cluster) backend() {
	// TODO: all operations that mutate Neighbors
	// need to come through this
	for f := range c.chF {
		f()
	}
}

func (c *cluster) verified(ir imageRecord) {
	c.chF <- func() {
		rv := append(c.recentlyVerified, ir)
		if len(rv) > 20 {
			rv = rv[1:]
		}
		c.recentlyVerified = rv
	}
}

func (c *cluster) Uploaded(ir imageRecord) {
	c.chF <- func() {
		rv := append(c.recentlyUploaded, ir)
		if len(rv) > 20 {
			rv = rv[1:]
		}
		c.recentlyUploaded = rv
	}
}

func (c *cluster) Stashed(ir imageRecord) {
	c.chF <- func() {
		rv := append(c.recentlyStashed, ir)
		if len(rv) > 20 {
			rv = rv[1:]
		}
		c.recentlyStashed = rv
	}
}

func (c *cluster) AddNeighbor(nd nodeData) {
	c.chF <- func() {
		c.neighbors[nd.UUID] = nd
	}
	numNeighbors.Add(1)
}

type gnresp struct {
	N []nodeData
}

func (c *cluster) GetNeighbors() []nodeData {
	r := make(chan gnresp)
	go func() {
		c.chF <- func() {
			neighbs := make([]nodeData, len(c.neighbors))
			var i = 0
			for _, value := range c.neighbors {
				neighbs[i] = value
				i++
			}
			r <- gnresp{neighbs}
		}
	}()
	resp := <-r
	return resp.N
}

func (c *cluster) RemoveNeighbor(nd nodeData) {
	c.chF <- func() {
		delete(c.neighbors, nd.UUID)
	}
	numNeighbors.Add(-1)
}

type fResp struct {
	N   *nodeData
	Err bool
}

func (c cluster) FindNeighborByUUID(uuid string) (*nodeData, bool) {
	r := make(chan fResp)
	go func() {
		c.chF <- func() {
			n, ok := c.neighbors[uuid]
			r <- fResp{&n, ok}
		}
	}()
	resp := <-r
	return resp.N, resp.Err
}

func (c *cluster) UpdateNeighbor(neighbor nodeData) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Nickname = neighbor.Nickname
			n.Location = neighbor.Location
			n.BaseURL = neighbor.BaseURL
			n.Writeable = neighbor.Writeable
			if neighbor.LastSeen.Sub(n.LastSeen) > 0 {
				n.LastSeen = neighbor.LastSeen
			}
			c.neighbors[neighbor.UUID] = n
		}
	}
}

func (c *cluster) FailedNeighbor(neighbor nodeData) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Writeable = false
			n.LastFailed = time.Now()
			c.neighbors[neighbor.UUID] = n
			neighborFailures.Add(1)
		}
	}
}

type listResp struct {
	Ns []nodeData
}

func (c cluster) NeighborsInclusive() []nodeData {
	r := make(chan listResp)
	go func() {
		c.chF <- func() {
			a := make([]nodeData, 1)
			a[0] = c.Myself

			neighbs := make([]nodeData, len(c.neighbors))
			var i = 0
			for _, value := range c.neighbors {
				neighbs[i] = value
				i++
			}

			a = append(a, neighbs...)
			r <- listResp{a}
		}
	}()
	resp := <-r
	return resp.Ns
}

func (c cluster) WriteableNeighbors() []nodeData {
	var all = c.NeighborsInclusive()
	var p []nodeData // == nil
	for _, i := range all {
		if i.Writeable {
			p = append(p, i)
		}
	}
	return p
}

type ringEntry struct {
	Node nodeData
	Hash string
}

type ringEntryList []ringEntry

func (p ringEntryList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ringEntryList) Len() int           { return len(p) }
func (p ringEntryList) Less(i, j int) bool { return p[i].Hash < p[j].Hash }

func (c cluster) Ring() ringEntryList {
	// TODO: cache the ring so we don't have to regenerate
	// every time. it only changes when a node joins or leaves
	return neighborsToRing(c.NeighborsInclusive())
}

func (c cluster) WriteRing() ringEntryList {
	return neighborsToRing(c.WriteableNeighbors())
}

func (c *cluster) Stash(ctx context.Context, ri imageSpecifier, sizeHints string, replication int, minReplication int, backend backend) []string {
	// we don't have the full-size, so check the cluster
	nodesToCheck := c.WriteOrder(ri.Hash.String())
	savedTo := make([]string, replication)
	var saveCount = 0
	// TODO: parallelize this
	for _, n := range nodesToCheck {
		// TODO: detect when the node to stash to is the current one
		// and just save directly instead of doing a POST to ourself
		if saveCount > 1 {
			// only have the first node on the list eagerly resize images
			sizeHints = ""
		}
		if n.Stash(ctx, ri, sizeHints, backend) {
			savedTo[saveCount] = n.Nickname
			saveCount++
			n.LastSeen = time.Now()
			c.UpdateNeighbor(n)
		} else {
			c.FailedNeighbor(n)
		}
		// TODO: if we've hit minReplication, we can return
		// immediately and leave any additional stash attempts
		// as background processes
		// that node didn't have it so we keep going
		if saveCount >= replication {
			// got as many as we need
			break
		}
	}
	return savedTo
}

func neighborsToRing(neighbors []nodeData) ringEntryList {
	keys := make(ringEntryList, REPLICAS*len(neighbors))
	for i := range neighbors {
		node := neighbors[i]
		nkeys := node.hashKeys()
		for j := range nkeys {
			keys[i*REPLICAS+j] = ringEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// returns the list of all nodes in the order
// that the given hash will choose to write to them
func (c cluster) WriteOrder(hash string) []nodeData {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (c cluster) ReadOrder(hash string) []nodeData {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.Ring())
}

func hashOrder(hash string, size int, ring []ringEntry) []nodeData {
	// our approach is to find the first bucket after our hash,
	// partition the ring on that and put the first part on the
	// end. Then go through and extract the ordering.

	// so, with a ring of [1,2,3,4,5,6,7,8,9,10]
	// and a hash of 7, we partition it into
	// [1,2,3,4,5,6] and [7,8,9,10]
	// then recombine them into
	// [7,8,9,10] + [1,2,3,4,5,6]
	// [7,8,9,10,1,2,3,4,5,6]
	var partitionIndex = 0
	for i, r := range ring {
		if r.Hash > hash {
			partitionIndex = i
			break
		}
	}
	// yay, slices
	reordered := make([]ringEntry, len(ring))
	reordered = append(ring[partitionIndex:], ring[:partitionIndex]...)

	results := make([]nodeData, size)
	var seen = map[string]bool{}
	var i = 0
	for _, r := range reordered {
		if !seen[r.Node.UUID] {
			results[i] = r.Node
			i++
			seen[r.Node.UUID] = true
		}
	}
	return results
}

// periodically pings all the known neighbors to gossip
// run this as a goroutine
func (c *cluster) Gossip(i, baseTime int, sl log.Logger) {
	sl.Log("level", "info", "msg", "starting gossiper")

	rand.Seed(int64(time.Now().Unix()) + int64(i))
	var jitter int
	firstRun := true

	for {
		// run forever
		for _, n := range c.GetNeighbors() {
			if n.UUID == c.Myself.UUID {
				// don't ping ourself
				continue
			}
			// avoid thundering herd
			jitter = rand.Intn(30)
			if firstRun {
				time.Sleep(time.Duration(jitter) * time.Second)
			} else {
				time.Sleep(time.Duration(baseTime+jitter) * time.Second)
			}
			firstRun = false
			sl.Log("level", "INFO",
				"action", "ping",
				"source", c.Myself.Nickname,
				"destination", n.Nickname)
			resp, err := n.Ping(c.Myself, sl)
			if err != nil {
				sl.Log("level", "INFO",
					"msg", "ping error",
					"source", c.Myself.Nickname,
					"destination", n.Nickname,
					"error", err.Error())
				c.FailedNeighbor(n)
				continue
			}
			// UUID and BaseURL must be the same
			n.Writeable = resp.Writeable
			n.Nickname = resp.Nickname
			n.Location = resp.Location
			n.LastSeen = time.Now()
			c.UpdateNeighbor(n)
			for _, neighbor := range resp.Neighbors {
				c.updateNeighbor(neighbor, sl)
			}
		}
	}
}

func (c *cluster) updateNeighbor(neighbor nodeData, sl log.Logger) {
	if neighbor.UUID == c.Myself.UUID {
		// as usual, skip ourself
		return
	}
	// TODO: convert these to a single atomic
	// UpdateOrAddNeighbor type operation
	if _, ok := c.FindNeighborByUUID(neighbor.UUID); ok {
		c.UpdateNeighbor(neighbor)
	} else {
		// heard about another node second hand
		sl.Log("level", "INFO", "msg", "adding neighbor via gossip")
		c.AddNeighbor(neighbor)
	}
}

func (c *cluster) RetrieveImage(ctx context.Context, ri *imageSpecifier) ([]byte, error) {
	// we don't have the full-size, so check the cluster
	nodesToCheck := c.ReadOrder(ri.Hash.String())
	// this is where we go down the list and ask the other
	// nodes for the image
	// TODO: parallelize this
	for _, n := range nodesToCheck {
		if n.UUID == c.Myself.UUID {
			// checking ourself would be silly
			continue
		}
		img, err := n.RetrieveImage(ctx, ri)
		if err == nil {
			// got it, return it
			return img, nil
		}
		// that node didn't have it so we keep going
	}
	return nil, errors.New("not found in the cluster")
}
