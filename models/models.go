package models

import (
	"time"
)

// what we know about a single node
// (ourself or another)
type NodeData struct {
	Nickname string
  UUID string
  BaseUrl string
  Location string
  Writeable bool
  LastSeen time.Time
  LastFailed time.Time
}

// represents what our Node nows about the world
// ie, itself and its neighbors
type World struct {
	Myself NodeData
	Neighbors []NodeData
	chF chan func()
}

func NewWorld(myself NodeData) *World {
	n := &World{myself, nil, make(chan func())}
	go n.backend()
	return n
}

func (n *World) backend() {
	for f := range n.chF {
		f()
	}
}

func (n *World) AddNeighbor(nd NodeData) {
	n.chF <- func() { n.Neighbors = append(n.Neighbors, nd) }
}



type ConfigData struct {
	Port int64
	UUID string
	Nickname string
	BaseUrl string
	Location string
	Writeable bool
	Neighbors []NodeData
}

func (c ConfigData) MyNode () NodeData {
	n := NodeData{
	Nickname: c.Nickname,
	UUID: c.UUID,
	BaseUrl: c.BaseUrl,
	Location: c.Location,
	Writeable: c.Writeable,
	}
	return n
}
