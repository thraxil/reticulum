package main

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

// RetrieveView encapsulates the business logic for retrieving images.
type RetrieveView struct {
	imageView *ImageView
	logger    log.Logger
}

// NewRetrieveView creates a new RetrieveView.
func NewRetrieveView(
	imageView *ImageView,
	logger log.Logger,
) *RetrieveView {
	return &RetrieveView{
		imageView: imageView,
		logger:    logger,
	}
}

// RetrieveImage retrieves and processes an image based on the image specifier parts.
// It returns the image data, Etag, and an error.
func (v *RetrieveView) RetrieveImage(ctx context.Context, hash, size, ext, ifNoneMatch string) ([]byte, string, error) {
	ahash, err := hashFromString(hash, "")
	if err != nil {
		return nil, "", fmt.Errorf("bad hash: %w", err)
	}
	extension := "." + ext

	ri := &imageSpecifier{
		ahash,
		resize.MakeSizeSpec(size),
		extension,
	}

	// Delegate to ImageView's GetImage logic
	imgData, etag, err := v.imageView.GetImage(ctx, ri)
	if err != nil {
		// Specific error handling for resize failures (if needed) can be added here.
		// For now, just pass through the error.
		return nil, "", err
	}

	return imgData, etag, nil
}
