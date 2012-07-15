package cluster

import (
	"../node"
	"fmt"
	"log"
	"log/syslog"
	"math/rand"
	"sort"
	"time"
)

// TODO: move this to a config
var REPLICAS = 16


// represents what our Node nows about the cluster
// ie, itself and its neighbors
type Cluster struct {
	Myself    node.NodeData
	Neighbors []node.NodeData
	chF       chan func()
}

func NewCluster(myself node.NodeData) *Cluster {
	n := &Cluster{Myself: myself, chF: make(chan func())}
	go n.backend()
	return n
}

func (n *Cluster) backend() {
	for f := range n.chF {
		f()
	}
}

func (n *Cluster) AddNeighbor(nd node.NodeData) {
	n.chF <- func() {
		n.Neighbors = append(n.Neighbors, nd)
	}
}

func (n Cluster) FindNeighborByUUID(uuid string) (*node.NodeData, bool) {
	for i := range n.Neighbors {
		if n.Neighbors[i].UUID == uuid {
			return &n.Neighbors[i], true
		}
	}
	return nil, false
}

func (n Cluster) NeighborsInclusive() []node.NodeData {
	a := make([]node.NodeData, len(n.Neighbors)+1)
	a[0] = n.Myself
	for i := range n.Neighbors {
		a[i+1] = n.Neighbors[i]
	}
	return a
}

func (n Cluster) WriteableNeighbors() []node.NodeData {
	var all = n.NeighborsInclusive()
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

func (n Cluster) Ring() RingEntryList {
	return neighborsToRing(n.NeighborsInclusive())
}

func (n Cluster) WriteRing() RingEntryList {
	return neighborsToRing(n.WriteableNeighbors())
}

func (cluster *Cluster) Stash(ahash string, filename string, replication int) []string {
	// we don't have the full-size, so check the cluster
	nodes_to_check := cluster.WriteOrder(ahash)
	saved_to := make([]string, replication)
	var save_count = 0
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		// TODO: detect when the node to stash to is the current one
		// and just save directly instead of doing a POST to ourself
		if n.Stash(filename) {
			saved_to[save_count] = n.Nickname
			save_count++
		}
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
func (n Cluster) WriteOrder(hash string) []node.NodeData {
	return hashOrder(hash, len(n.Neighbors)+1, n.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (n Cluster) ReadOrder(hash string) []node.NodeData {
	return hashOrder(hash, len(n.Neighbors)+1, n.Ring())
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
func (c *Cluster) Gossip(i, base_time int) {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	sl.Info("starting gossiper")

	rand.Seed(int64(time.Now().Unix()) + int64(i))
	var jitter int
	for {
		// run forever
		for _, n := range c.Neighbors {
			if n.UUID == c.Myself.UUID {
				// don't ping ourself
				continue
			}
			// avoid thundering herd
			jitter = rand.Intn(30)
			time.Sleep(time.Duration(base_time + jitter) * time.Second)
			sl.Info(fmt.Sprintf("node %s pinging %s",c.Myself.Nickname,n.Nickname))
			resp := n.Ping(c.Myself)
			// UUID and BaseUrl must be the same
			n.Writeable = resp.Writeable
			n.Nickname = resp.Nickname
			n.Location = resp.Location
			for _, neighbor := range resp.Neighbors {
				if neighbor.UUID == c.Myself.UUID {
					// as usual, skip ourself
					continue
				}
				if existing_neighbor, ok := c.FindNeighborByUUID(neighbor.UUID); ok {
					existing_neighbor.Nickname = neighbor.Nickname
					existing_neighbor.Location = neighbor.Location
					existing_neighbor.BaseUrl = neighbor.BaseUrl
					existing_neighbor.Writeable = neighbor.Writeable
				} else {
					// heard about another node second hand
					fmt.Println("adding neighbor via gossip")
					c.AddNeighbor(neighbor)
				}
			}
		}
	}
}
