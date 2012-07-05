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

// represents what our Node nows about the world
// ie, itself and its neighbors
type World struct {
	Myself    NodeData
	Neighbors []NodeData
	chF       chan func()
}

func NewWorld(myself NodeData) *World {
	n := &World{Myself: myself, chF: make(chan func())}
	go n.backend()
	return n
}

func (n *World) backend() {
	for f := range n.chF {
		f()
	}
}

func (n *World) AddNeighbor(nd NodeData) {
	n.chF <- func() {
		n.Neighbors = append(n.Neighbors, nd)
	}
}

func (n World) FindNeighborByUUID(uuid string) (*NodeData, bool) {
	for i := range n.Neighbors {
		if n.Neighbors[i].UUID == uuid {
			return &n.Neighbors[i], true
		}
	}
	return nil, false
}

type ConfigData struct {
	Port      int64
	UUID      string
	Nickname  string
	BaseUrl   string
	Location  string
	Writeable bool
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
