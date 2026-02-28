package main

import (
	"bytes"

	"context"

	"crypto/sha1"

	"fmt"

	"github.com/go-kit/log"
)

// ImageView encapsulates the business logic for serving images.
type ImageView struct {
	cluster    Cluster
	backend    Backend
	siteConfig *siteConfig // Still need siteConfig for some values, will refactor later
	channels   sharedChannels
	logger     log.Logger
}

// NewImageView creates a new ImageView.
func NewImageView(
	cluster Cluster,
	backend Backend,
	siteConfig *siteConfig,
	channels sharedChannels,
	logger log.Logger,
) *ImageView {
	return &ImageView{
		cluster:    cluster,
		backend:    backend,
		siteConfig: siteConfig,
		channels:   channels,
		logger:     logger,
	}
}

// GetImage retrieves and processes an image based on the image specifier.
// It returns the image data, Etag, and an error.
func (v *ImageView) GetImage(ctx context.Context, ri *imageSpecifier) ([]byte, string, error) {
	// Try to serve directly from local backend
	contents, err := v.backend.Read(*ri)
	if err == nil {
		// We have it, calculate Etag and return
		etag := fmt.Sprintf("%x", sha1.Sum(contents))
		return contents, etag, nil
	}

	// If not found locally, check if full-size is available locally
	if !v.haveImageFullsizeLocally(ri) {
		// If full-size not local, try to retrieve from cluster
		imgData, err := v.cluster.RetrieveImage(ctx, ri)
		if err != nil {
			return nil, "", err // Not found in cluster either
		}
		etag := fmt.Sprintf("%x", sha1.Sum(imgData))
		return imgData, etag, nil
	}

	// We have the full-size, but not the scaled one, so resize it
	if !v.locallyWriteable() {
		// If not writeable, let another node in the cluster handle the scaling
		imgData, err := v.cluster.RetrieveImage(ctx, ri) // Request scaled image from cluster
		if err != nil {
			return nil, "", err
		}
		etag := fmt.Sprintf("%x", sha1.Sum(imgData))
		return imgData, etag, nil
	}

	// Resize locally
	result := v.makeResizeJob(ri)
	if !result.Success {
		resizeFailures.Add(1) // Global expvar, needs to be handled
		return nil, "", fmt.Errorf("could not resize image")
	}
	servedScaled.Add(1) // Global expvar, needs to be handled

	var buf bytes.Buffer
	enc := extencoders[ri.Extension] // Global map, needs to be handled
	_ = enc(&buf, *result.OutputImage)
	contents = buf.Bytes()
	etag := fmt.Sprintf("%x", sha1.Sum(contents))

	// Write scaled image to local backend
	if err := v.backend.writeLocalType(*ri, *result.OutputImage, enc); err != nil {
		_ = v.logger.Log("level", "ERR", "msg", "error writing scaled image locally", "error", err)
	}

	return contents, etag, nil
}

func (v *ImageView) locallyWriteable() bool {
	return v.cluster.GetMyself().Writeable
}

func (v *ImageView) haveImageFullsizeLocally(ri *imageSpecifier) bool {
	_, err := v.backend.Read(ri.fullVersion())
	return err == nil
}

func (v *ImageView) makeResizeJob(ri *imageSpecifier) resizeResponse {
	c := make(chan resizeResponse)
	fmt.Println(ri.fullSizePath(v.siteConfig.UploadDirectory))
	v.channels.ResizeQueue <- resizeRequest{ri.fullSizePath(v.siteConfig.UploadDirectory), ri.Extension, ri.Size.String(), c}
	resizeQueueLength.Add(1) // Global expvar, needs to be handled
	result := <-c
	resizeQueueLength.Add(-1) // Global expvar, needs to be handled
	return result
}
