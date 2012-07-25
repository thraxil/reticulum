package models

import (
	"../node"
	"../resize_worker"
)

// the structure of the config.json file
// where config info is stored
type ConfigData struct {
	Port             int64
	UUID             string
	Nickname         string
	BaseUrl          string
	Location         string
	Writeable        bool
	NumResizeWorkers int
	UploadKeys       []string
	UploadDirectory  string
	Neighbors        []node.NodeData
	Replication      int
	GossiperSleep    int
	VerifierSleep    int
}

func (c ConfigData) MyNode() node.NodeData {
	n := node.NodeData{
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
	numWorkers := c.NumResizeWorkers
	if numWorkers < 1 {
		// come on! we need at least one
		numWorkers = 1
	}
	replication := c.Replication
	if replication < 1 {
		replication = 1
	}
	gossiper_sleep := c.GossiperSleep
	if gossiper_sleep < 1 {
		// default to 60 seconds
		gossiper_sleep = 60
	}
	verifier_sleep := c.VerifierSleep
	if verifier_sleep < 1 {
		verifier_sleep = 300
	}
	return SiteConfig{
		Port:             c.Port,
		UploadKeys:       c.UploadKeys,
		UploadDirectory:  c.UploadDirectory,
		NumResizeWorkers: numWorkers,
		Replication:      replication,
		GossiperSleep:    gossiper_sleep,
		VerifierSleep:    verifier_sleep,
	}
}

// basically a subset of ConfigData, that is just
// the general administrative stuff
type SiteConfig struct {
	Port             int64
	UploadKeys       []string
	UploadDirectory  string
	NumResizeWorkers int
	Replication      int
	GossiperSleep    int
	VerifierSleep    int
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

type SharedChannels struct {
	ResizeQueue chan resize_worker.ResizeRequest
}
