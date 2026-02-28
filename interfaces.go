package main

import (
	"context"
	"image"
	"io"
)

// Backend is an interface for storing and retrieving images.
type Backend interface {
	Read(spec imageSpecifier) ([]byte, error)
	WriteFull(spec imageSpecifier, reader io.ReadCloser) error
	writeLocalType(spec imageSpecifier, img image.Image, enc encfunc) error
	fullPath(ri imageSpecifier) string
	Delete(img imageSpecifier) error
}

// Cluster is an interface for interacting with the cluster.
type Cluster interface {
	RetrieveImage(ctx context.Context, ri *imageSpecifier) ([]byte, error)
	Stash(ctx context.Context, ri imageSpecifier, sizeHints string, replication int, minReplication int, backend Backend) []string
	Uploaded(r imageRecord)
	GetNeighbors() []nodeData
	FindNeighborByUUID(uuid string) (*nodeData, bool)

	// these are used by the announce handler, and maybe shouldn't be on this interface
	UpdateNeighbor(n nodeData)
	AddNeighbor(n nodeData)
	Stashed(r imageRecord)
	GetMyself() nodeData
	GetRecentlyVerified() []imageRecord
	GetRecentlyUploaded() []imageRecord
	GetRecentlyStashed() []imageRecord
}
