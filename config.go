package main

import ()

// the structure of the config.json file
// where config info is stored
type ConfigData struct {
	Port                   int64
	UUID                   string
	Nickname               string
	BaseUrl                string
	Location               string
	Writeable              bool
	NumResizeWorkers       int
	UploadKeys             []string
	UploadDirectory        string
	Neighbors              []NodeData
	Replication            int
	MinReplication         int
	MaxReplication         int
	GossiperSleep          int
	VerifierSleep          int
	ImageMagickConvertPath string
	GoMaxProcs             int
	GroupcacheUrl          string
	GroupcacheSize         int64
}

func (c ConfigData) MyNode() NodeData {
	n := NodeData{
		Nickname:      c.Nickname,
		UUID:          c.UUID,
		BaseUrl:       c.BaseUrl,
		Location:      c.Location,
		Writeable:     c.Writeable,
		GroupcacheUrl: c.GroupcacheUrl,
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
	// these default to replication if not set
	min_replication := c.MinReplication
	if min_replication < 1 {
		min_replication = replication
	}
	max_replication := c.MaxReplication
	if max_replication < 1 {
		max_replication = replication
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

	convert_path := c.ImageMagickConvertPath
	if convert_path == "" {
		convert_path = "/usr/bin/convert"
	}

	go_max_procs := c.GoMaxProcs
	if go_max_procs < 1 {
		go_max_procs = 1
	}

	groupcache_size := c.GroupcacheSize
	if groupcache_size < 1 {
		groupcache_size = 64 << 20
	}

	return SiteConfig{
		Port:                   c.Port,
		UploadKeys:             c.UploadKeys,
		UploadDirectory:        c.UploadDirectory,
		NumResizeWorkers:       numWorkers,
		Replication:            replication,
		MinReplication:         min_replication,
		MaxReplication:         max_replication,
		GossiperSleep:          gossiper_sleep,
		VerifierSleep:          verifier_sleep,
		ImageMagickConvertPath: convert_path,
		GoMaxProcs:             c.GoMaxProcs,
		Writeable:              c.Writeable,
		GroupcacheUrl:          c.GroupcacheUrl,
		GroupcacheSize:         c.GroupcacheSize,
	}
}

// basically a subset of ConfigData, that is just
// the general administrative stuff
type SiteConfig struct {
	Port                   int64
	UploadKeys             []string
	UploadDirectory        string
	NumResizeWorkers       int
	Replication            int
	MinReplication         int
	MaxReplication         int
	GossiperSleep          int
	VerifierSleep          int
	ImageMagickConvertPath string
	GoMaxProcs             int
	Writeable              bool
	GroupcacheUrl          string
	GroupcacheSize         int64
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
