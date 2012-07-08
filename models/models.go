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
	return neighborsToRing(n.NeighborsInclusive())
}

func (n Cluster) WriteRing() RingEntryList {
	return neighborsToRing(n.WriteableNeighbors())
}

func neighborsToRing(neighbors []NodeData) RingEntryList {
	keys := make(RingEntryList, REPLICAS*len(neighbors))
	for i := range neighbors {
		node := neighbors[i]
		nkeys := node.HashKeys()
		for j := range nkeys {
			keys[i*REPLICAS + j] = RingEntry{Node: node, Hash: nkeys[j]}
		}
	}
	sort.Sort(keys)
	return keys
}

// returns the list of all nodes in the order
// that the given hash will choose to write to them
func (n Cluster) WriteOrder(hash string) []NodeData {
	return hashOrder(hash, len(n.Neighbors) + 1, n.WriteRing())
}

// returns the list of all nodes in the order
// that the given hash will choose to try to read from them
func (n Cluster) ReadOrder(hash string) []NodeData {
	return hashOrder(hash, len(n.Neighbors) + 1, n.Ring())
}

func hashOrder(hash string, size int, ring []RingEntry) []NodeData {
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

	results := make([]NodeData, size)
	var seen = map[string] bool {}
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
