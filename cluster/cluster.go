package cluster

import (
	"fmt"
	"github.com/thraxil/reticulum/node"
	"log/syslog"
	"math/rand"
	"sort"
	"time"
)

// TODO: move this to a config
var REPLICAS = 16

// represents what our Node nows about the cluster
// ie, itself and its neighbors
// TODO: we do a lot of lookups of neighbors by UUID
// there should probably be a map for that so we don't
// have to run through the whole list every time
type Cluster struct {
	Myself    node.NodeData
	neighbors map[string]node.NodeData
	chF       chan func()
}

func NewCluster(myself node.NodeData) *Cluster {
	c := &Cluster{
		Myself:    myself,
		neighbors: make(map[string]node.NodeData),
		chF:       make(chan func()),
	}
	go c.backend()
	return c
}

func (c *Cluster) backend() {
	// TODO: all operations that mutate Neighbors
	// need to come through this
	for f := range c.chF {
		f()
	}
}

func (c *Cluster) AddNeighbor(nd node.NodeData) {
	c.chF <- func() {
		c.neighbors[nd.UUID] = nd
	}
}

type gnresp struct {
	N []node.NodeData
}

func (c *Cluster) GetNeighbors() []node.NodeData {
	r := make(chan gnresp)
	go func() {
		c.chF <- func() {
			neighbs := make([]node.NodeData, len(c.neighbors))
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

func (c *Cluster) RemoveNeighbor(nd node.NodeData) {
	c.chF <- func() {
		delete(c.neighbors, nd.UUID)
	}
}

type fResp struct {
	N   *node.NodeData
	Err bool
}

func (c Cluster) FindNeighborByUUID(uuid string) (*node.NodeData, bool) {
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

func (c *Cluster) UpdateNeighbor(neighbor node.NodeData) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Nickname = neighbor.Nickname
			n.Location = neighbor.Location
			n.BaseUrl = neighbor.BaseUrl
			n.Writeable = neighbor.Writeable
			if neighbor.LastSeen.Sub(n.LastSeen) > 0 {
				n.LastSeen = neighbor.LastSeen
			}
			c.neighbors[neighbor.UUID] = n
		}
	}
}

func (c *Cluster) FailedNeighbor(neighbor node.NodeData) {
	c.chF <- func() {
		if n, ok := c.neighbors[neighbor.UUID]; ok {
			n.Writeable = false
			n.LastFailed = time.Now()
			c.neighbors[neighbor.UUID] = n
		}
	}
}

type listResp struct {
	Ns []node.NodeData
}

func (c Cluster) NeighborsInclusive() []node.NodeData {
	r := make(chan listResp)
	go func() {
		c.chF <- func() {
			a := make([]node.NodeData, 1)
			a[0] = c.Myself

			neighbs := make([]node.NodeData, len(c.neighbors))
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

func (c Cluster) WriteableNeighbors() []node.NodeData {
	var all = c.NeighborsInclusive()
	var p []node.NodeData // == nil
	for _, i := range all {
		if i.Writeable {
			p = append(p, i)
		}
	}
	return p
}

type RingEntry struct {
	Node node.NodeData
	Hash string // the hash
}

type RingEntryList []RingEntry

func (p RingEntryList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p RingEntryList) Len() int           { return len(p) }
func (p RingEntryList) Less(i, j int) bool { return p[i].Hash < p[j].Hash }

func (c Cluster) Ring() RingEntryList {
	// TODO: cache the ring so we don't have to regenerate
	// every time. it only changes when a node joins or leaves
	return neighborsToRing(c.NeighborsInclusive())
}

func (c Cluster) WriteRing() RingEntryList {
	return neighborsToRing(c.WriteableNeighbors())
}

func (cluster *Cluster) Stash(ahash string, filename string, size_hints string, replication int, min_replication int) []string {
	// we don't have the full-size, so check the cluster
	nodes_to_check := cluster.WriteOrder(ahash)
	saved_to := make([]string, replication)
	var save_count = 0
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		// TODO: detect when the node to stash to is the current one
		// and just save directly instead of doing a POST to ourself
		if save_count > 1 {
			// only have the first node on the list eagerly resize images
			size_hints = ""
		}
		if n.Stash(filename, size_hints) {
			saved_to[save_count] = n.Nickname
			save_count++
			n.LastSeen = time.Now()
			cluster.UpdateNeighbor(n)
		} else {
			cluster.FailedNeighbor(n)
		}
		// TODO: if we've hit min_replication, we can return
		// immediately and leave any additional stash attempts
		// as background processes
		// that node didn't have it so we keep going
		if save_count >= replication {
			// got as many as we need
			break
		}
	}
	return saved_to
}

func neighborsToRing(neighbors []node.NodeData) RingEntryList {
	keys := make(RingEntryList, REPLICAS*len(neighbors))
	for i := range neighbors {
		node := neighbors[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*REPLICAS+j] = RingEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// returns the list of all nodes in the order
// that the given hash will choose to write to them
func (c Cluster) WriteOrder(hash string) []node.NodeData {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (c Cluster) ReadOrder(hash string) []node.NodeData {
	return hashOrder(hash, len(c.GetNeighbors())+1, c.Ring())
}

func hashOrder(hash string, size int, ring []RingEntry) []node.NodeData {
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
	reordered := make([]RingEntry, len(ring))
	reordered = append(ring[partitionIndex:], ring[:partitionIndex]...)

	results := make([]node.NodeData, size)
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
func (c *Cluster) Gossip(i, base_time int, sl *syslog.Writer) {
	sl.Info("starting gossiper")

	rand.Seed(int64(time.Now().Unix()) + int64(i))
	var jitter int
	for {
		// run forever
		for _, n := range c.GetNeighbors() {
			if n.UUID == c.Myself.UUID {
				// don't ping ourself
				continue
			}
			// avoid thundering herd
			jitter = rand.Intn(30)
			time.Sleep(time.Duration(base_time+jitter) * time.Second)
			sl.Info(fmt.Sprintf("node %s pinging %s", c.Myself.Nickname, n.Nickname))
			resp, err := n.Ping(c.Myself)
			if err != nil {
				sl.Info(fmt.Sprintf("error on node %s pinging %s", c.Myself.Nickname, n.Nickname))
				c.FailedNeighbor(n)
				continue
			}
			// UUID and BaseUrl must be the same
			n.Writeable = resp.Writeable
			n.Nickname = resp.Nickname
			n.Location = resp.Location
			n.LastSeen = time.Now()
			c.UpdateNeighbor(n)
			for _, neighbor := range resp.Neighbors {
				if neighbor.UUID == c.Myself.UUID {
					// as usual, skip ourself
					continue
				}
				// TODO: convert these to a single atomic
				// UpdateOrAddNeighbor type operation
				if _, ok := c.FindNeighborByUUID(neighbor.UUID); ok {
					c.UpdateNeighbor(neighbor)
				} else {
					// heard about another node second hand
					sl.Info("adding neighbor via gossip")
					c.AddNeighbor(neighbor)
				}
			}
		}
	}
}
