package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-kit/log"
)

// RetrieveInfoView encapsulates the business logic for retrieving image info.
type RetrieveInfoView struct {
	cluster    Cluster
	siteConfig *siteConfig
	logger     log.Logger
}

// NewRetrieveInfoView creates a new RetrieveInfoView.
func NewRetrieveInfoView(
	cluster Cluster,
	siteConfig *siteConfig,
	logger log.Logger,
) *RetrieveInfoView {
	return &RetrieveInfoView{
		cluster:    cluster,
		siteConfig: siteConfig,
		logger:     logger,
	}
}

// GetImageInfo retrieves and processes image information.
// It returns the JSON marshalled imageInfoResponse or an error.
func (v *RetrieveInfoView) GetImageInfo(hash, size, ext string) ([]byte, error) {
	ahash, err := hashFromString(hash, "")
	if err != nil {
		return nil, fmt.Errorf("bad hash: %w", err)
	}
	extension := "." + ext
	var local = true
	baseDir := v.siteConfig.UploadDirectory + ahash.AsPath()
	path := baseDir + "/full" + extension
	_, err = os.Open(path)
	if err != nil {
		local = false
	}

	// if we aren't writeable, we can't resize locally
	// let them know this as early as possible
	n := v.cluster.GetMyself()
	if size != "full" && !n.Writeable {
		// anything other than full-size, we can't do
		// if we don't have it already
		_, err = os.Open(baseDir + "/" + size + extension)
		if err != nil {
			local = false
		}
	}

	b, err := json.Marshal(imageInfoResponse{ahash.String(), extension, local})
	if err != nil {
		_ = v.logger.Log("level", "ERR", "msg", "error marshalling image info", "error", err.Error())
		return nil, fmt.Errorf("failed to marshal image info: %w", err)
	}
	return b, nil
}
