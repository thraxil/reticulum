package models

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"time"
)

// what we know about a single node
// (ourself or another)
type NodeData struct {
	Nickname   string
	UUID       string
	BaseUrl    string
	Location   string
	Writeable  bool
	LastSeen   time.Time
	LastFailed time.Time
}

var REPLICAS = 16;

func (n NodeData) String() string {
	return "Node - nickname: " + n.Nickname + " UUID: " + n.UUID
}

func (n NodeData) HashKeys() []string {
	keys := make([]string, REPLICAS)
	for i := range keys {
		h := sha1.New()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = string(fmt.Sprintf("%x", h.Sum(nil)))
	}
	return keys
}

func (n NodeData) IsCurrent() bool {
	return n.LastSeen.Unix() > n.LastFailed.Unix()
}

// represents what our Node nows about the cluster
// ie, itself and its neighbors
type Cluster struct {
	Myself    NodeData
	Neighbors []NodeData
	chF       chan func()
}

func NewCluster(myself NodeData) *Cluster {
	n := &Cluster{Myself: myself, chF: make(chan func())}
	go n.backend()
	return n
}

func (n *Cluster) backend() {
	for f := range n.chF {
		f()
	}
}

func (n *Cluster) AddNeighbor(nd NodeData) {
	n.chF <- func() {
		n.Neighbors = append(n.Neighbors, nd)
	}
}

func (n Cluster) FindNeighborByUUID(uuid string) (*NodeData, bool) {
	for i := range n.Neighbors {
		if n.Neighbors[i].UUID == uuid {
			return &n.Neighbors[i], true
		}
	}
	return nil, false
}

func (n Cluster) NeighborsInclusive() []NodeData {
	a := make([]NodeData, len(n.Neighbors) + 1)
	a[0] = n.Myself
	for i := range n.Neighbors {
		a[i+1] = n.Neighbors[i]
	}
	return a
}

func (n Cluster) WriteableNeighbors() []NodeData {
	var all = n.NeighborsInclusive()
	var p []NodeData // == nil
  for _, i := range all {
    if i.Writeable {
      p = append(p, i)
    }
  }
  return p
}

type RingEntry struct {
	Node NodeData
	Hash string // the hash
}

type RingEntryList []RingEntry
func (p RingEntryList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p RingEntryList) Len() int { return len(p) }
func (p RingEntryList) Less(i, j int) bool { return p[i].Hash < p[j].Hash }

func (n Cluster) Ring() RingEntryList {
	allnodes := n.NeighborsInclusive()
	keys := make(RingEntryList, REPLICAS*len(allnodes))
	for i := range allnodes {
		node := allnodes[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*REPLICAS + j] = RingEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

func (n Cluster) WriteRing() RingEntryList {
	allnodes := n.WriteableNeighbors()
	keys := make(RingEntryList, REPLICAS*len(allnodes))
	for i := range allnodes {
		node := allnodes[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*REPLICAS + j] = RingEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// def write_order(image_hash):
//     wr = deque(write_ring())
//     nodes = []
//     appending = False
//     seen = dict()
//     while len(wr) > 0:
//         # get the first element
//         (k,n) = wr.popleft()
//         if appending or image_hash > k:
//             if n.uuid not in seen:
//                 nodes.append(n)
//                 seen[n.uuid] = True
//             appending = True
//         else:
//             # put it back on
//             wr.append((k,n))
//     return nodes

// def read_order(image_hash):
//     r = deque(ring())
//     nodes = []
//     appending = False
//     seen = dict()
//     while len(r) > 0:
//         # get the first element
//         (k,n) = r.popleft()
//         if appending or image_hash > k:
//             if n.uuid not in seen:
//                 nodes.append(n)
//                 seen[n.uuid] = True
//             appending = True
//         else:
//             # put it back on
//             r.append((k,n))
//     return nodes


// the structure of the config.json file
// where config info is stored
type ConfigData struct {
	Port      int64
	UUID      string
	Nickname  string
	BaseUrl   string
	Location  string
	Writeable bool
	UploadKeys []string
	UploadDirectory string
	Neighbors []NodeData
}

func (c ConfigData) MyNode() NodeData {
	n := NodeData{
		Nickname:  c.Nickname,
		UUID:      c.UUID,
		BaseUrl:   c.BaseUrl,
		Location:  c.Location,
		Writeable: c.Writeable,
	}
	return n
}

func (c ConfigData) MyConfig() SiteConfig {
	// todo: defaults should go here
	// todo: normalize uploaddirectory trailing slash
	return SiteConfig{
	Port: c.Port,
	UploadKeys: c.UploadKeys,
	UploadDirectory: c.UploadDirectory,
	}
}

// basically a subset of ConfigData, that is just
// the general administrative stuff
type SiteConfig struct {
	Port int64
	UploadKeys []string
	UploadDirectory string
}

func (s SiteConfig) KeyRequired() bool {
	return len(s.UploadKeys) > 0
}

func (s SiteConfig) ValidKey(key string) bool {
	for i := range s.UploadKeys {
		if key == s.UploadKeys[i] {
			return true
		}
	}
	return false
}
