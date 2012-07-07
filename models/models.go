package models

import (
	"crypto/sha1"
	"fmt"
	"io"
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

func (n NodeData) String() string {
	return "Node - nickname: " + n.Nickname + " UUID: " + n.UUID
}

func (n NodeData) HashKeys() []string {
	keys := make([]string, 128)
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
