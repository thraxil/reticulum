package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/go-kit/log"
)

// StashView encapsulates the business logic for stashing images.
type StashView struct {
	cluster    Cluster
	backend    Backend
	siteConfig *siteConfig
	channels   sharedChannels
	logger     log.Logger
}

// NewStashView creates a new StashView.
func NewStashView(
	cluster Cluster,
	backend Backend,
	siteConfig *siteConfig,
	channels sharedChannels,
	logger log.Logger,
) *StashView {
	return &StashView{
		cluster:    cluster,
		backend:    backend,
		siteConfig: siteConfig,
		channels:   channels,
		logger:     logger,
	}
}

// StashImage handles the image stashing process.
// It returns "ok" on success or an error.
func (v *StashView) StashImage(
	ctx context.Context,
	imageFile io.ReadSeeker, // io.ReadSeeker for seek operations
	fileHeader *multipart.FileHeader,
	sizeHints string,
) (string, error) {
	n := v.cluster.GetMyself()
	if !n.Writeable {
		return "", fmt.Errorf("non-writeable node")
	}

	// Determine mimetype and extension
	mimetype := fileHeader.Header.Get("Content-Type")
	ext, ok := mimeexts[mimetype]
	if !ok {
		return "", fmt.Errorf("unsupported image type: %s", mimetype)
	}

	h := sha1.New()
	_, _ = io.Copy(h, imageFile)
	ahash, err := hashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
	if err != nil {
		return "", fmt.Errorf("bad hash: %w", err)
	}

	_, _ = imageFile.Seek(0, io.SeekStart)

	path := v.siteConfig.UploadDirectory + "/" + ahash.AsPath()
	_ = os.MkdirAll(path, 0755)
	fullpath := path + "/full" + ext
	f, err := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		_ = v.logger.Log("level", "ERR", "msg", "error opening file for stashing", "error", err.Error())
		return "", fmt.Errorf("failed to open file for stashing: %w", err)
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, imageFile)
	if err != nil {
		_ = v.logger.Log("level", "ERR", "msg", "error copying file for stashing", "error", err.Error())
		return "", fmt.Errorf("failed to copy file for stashing: %w", err)
	}

	// do any eager resizing in the background
	go func() {
		sizes := strings.Split(sizeHints, ",")
		for _, size := range sizes {
			if size == "" {
				continue
			}
			c := make(chan resizeResponse)
			v.channels.ResizeQueue <- resizeRequest{fullpath, ext, size, c}
			result := <-c
			if !result.Success {
				_ = v.logger.Log("level", "ERR", "msg", "could not pre-resize")
			}
		}
	}()
	v.cluster.Stashed(imageRecord{*ahash, ext})
	return "ok", nil
}
