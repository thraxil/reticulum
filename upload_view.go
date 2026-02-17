package main

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

// UploadView encapsulates the business logic for uploading images.
type UploadView struct {
	cluster    Cluster
	backend    Backend
	siteConfig *siteConfig
	logger     log.Logger
}

// NewUploadView creates a new UploadView.
func NewUploadView(
	cluster Cluster,
	backend Backend,
	siteConfig *siteConfig,
	logger log.Logger,
) *UploadView {
	return &UploadView{
		cluster:    cluster,
		backend:    backend,
		siteConfig: siteConfig,
		logger:     logger,
	}
}

// UploadImage handles the image upload process.
// It returns the JSON marshalled imageData or an error.
func (v *UploadView) UploadImage(
	ctx context.Context,
	key string,
	imageFile io.ReadSeeker, // io.ReadSeeker for seek operations
	fileHeader *multipart.FileHeader,
	sizeHints string,
) ([]byte, error) {
	// Key validation
	if v.siteConfig.KeyRequired() {
		if !v.siteConfig.ValidKey(key) {
			return nil, fmt.Errorf("invalid upload key")
		}
	}

	// Hashing the image content
	h := sha1.New()
	_, _ = io.Copy(h, imageFile)
	ahash, err := hashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
	if err != nil {
		return nil, fmt.Errorf("bad hash: %w", err)
	}

	// Reset imageFile to the beginning for subsequent reads
	_, _ = imageFile.Seek(0, io.SeekStart)

	// Determine mimetype and extension
	mimetype := fileHeader.Header.Get("Content-Type")
	if mimetype == "" {
		mimetype = "image/jpeg" // Default to jpg
	}
	ext, ok := mimeexts[mimetype]
	if !ok {
		ext = "jpg" // Unknown mimetype, default to jpg
	}
	ri := imageSpecifier{
		ahash,
		resize.MakeSizeSpec("full"),
		"." + ext,
	}

	// Write full image to backend
	if err := v.backend.WriteFull(ri, imageFile.(io.ReadCloser)); err != nil { // Type assertion for ReadCloser
		_ = v.logger.Log("level", "ERR", "msg", "error writing full image to backend", "error", err.Error())
		return nil, fmt.Errorf("failed to write image to backend: %w", err)
	}

	// Stash to other nodes in the cluster
	nodes := v.cluster.Stash(
		ctx, ri, sizeHints, v.siteConfig.Replication,
		v.siteConfig.MinReplication, v.backend,
	)

	// Prepare response data
	id := imageData{
		Hash:      ahash.String(),
		Extension: ext,
		FullURL:   "/image/" + ahash.String() + "/full/image." + ext,
		Satisfied: len(nodes) >= v.siteConfig.MinReplication,
		Nodes:     nodes,
	}
	b, err := json.Marshal(id)
	if err != nil {
		_ = v.logger.Log("level", "ERR", "msg", "error marshalling image data", "error", err.Error())
		return nil, fmt.Errorf("failed to marshal image data: %w", err)
	}

	v.cluster.Uploaded(imageRecord{*ahash, "." + ext})

	return b, nil
}
