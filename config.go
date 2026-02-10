package main

// the structure of the config.json file
// where config info is stored
type configData struct {
	Port                   int64
	UUID                   string
	Nickname               string
	BaseURL                string `json:"BaseUrl"`
	Location               string
	Writeable              bool
	NumResizeWorkers       int
	UploadKeys             []string
	UploadDirectory        string
	Neighbors              []nodeData
	Replication            int
	MinReplication         int
	MaxReplication         int
	GossiperSleep          int
	VerifierSleep          int
	GoMaxProcs             int
}

func (c configData) MyNode() nodeData {
	n := nodeData{
		Nickname:  c.Nickname,
		UUID:      c.UUID,
		BaseURL:   c.BaseURL,
		Location:  c.Location,
		Writeable: c.Writeable,
	}
	return n
}

func (c configData) MyConfig() siteConfig {
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
	minReplication := c.MinReplication
	if minReplication < 1 {
		minReplication = replication
	}
	maxReplication := c.MaxReplication
	if maxReplication < 1 {
		maxReplication = replication
	}
	gossiperSleep := c.GossiperSleep
	if gossiperSleep < 1 {
		// default to 60 seconds
		gossiperSleep = 60
	}
	verifierSleep := c.VerifierSleep
	if verifierSleep < 1 {
		verifierSleep = 300
	}

	goMaxProcs := c.GoMaxProcs
	if goMaxProcs < 1 {
		goMaxProcs = 1
	}

	b := newDiskBackend(c.UploadDirectory)

	return siteConfig{
		Port:                   c.Port,
		UploadKeys:             c.UploadKeys,
		UploadDirectory:        c.UploadDirectory,
		NumResizeWorkers:       numWorkers,
		Replication:            replication,
		MinReplication:         minReplication,
		MaxReplication:         maxReplication,
		GossiperSleep:          gossiperSleep,
		VerifierSleep:          verifierSleep,
		GoMaxProcs:             goMaxProcs,
		Writeable:              c.Writeable,
		Backend:                b,
	}
}

// basically a subset of configData, that is just
// the general administrative stuff
type siteConfig struct {
	Port                   int64
	UploadKeys             []string
	UploadDirectory        string
	NumResizeWorkers       int
	Replication            int
	MinReplication         int
	MaxReplication         int
	GossiperSleep          int
	VerifierSleep          int
	GoMaxProcs             int
	Writeable              bool
	Backend                backend
}

func (s siteConfig) KeyRequired() bool {
	return len(s.UploadKeys) > 0
}

func (s siteConfig) ValidKey(key string) bool {
	for i := range s.UploadKeys {
		if key == s.UploadKeys[i] {
			return true
		}
	}
	return false
}
