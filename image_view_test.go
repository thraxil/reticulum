package main

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

// mockCluster is a mock implementation of the Cluster interface for testing.
type mockCluster struct {
	RetrieveImageFunc       func(ctx context.Context, ri *imageSpecifier) ([]byte, error)
	GetMyselfFunc            func() nodeData
	StashFunc               func(ctx context.Context, ri imageSpecifier, sizeHints string, replication int, minReplication int, backend Backend) []string
	UploadedFunc            func(r imageRecord)
	GetNeighborsFunc        func() []nodeData
	FindNeighborByUUIDFunc  func(uuid string) (*nodeData, bool)
	UpdateNeighborFunc      func(n nodeData)
	AddNeighborFunc         func(n nodeData)
	StashedFunc             func(r imageRecord)
	GetRecentlyVerifiedFunc func() []imageRecord
	GetRecentlyUploadedFunc func() []imageRecord
	GetRecentlyStashedFunc  func() []imageRecord
}

func (m *mockCluster) RetrieveImage(ctx context.Context, ri *imageSpecifier) ([]byte, error) {
	if m.RetrieveImageFunc != nil {
		return m.RetrieveImageFunc(ctx, ri)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCluster) GetMyself() nodeData {
	if m.GetMyselfFunc != nil {
		return m.GetMyselfFunc()
	}
	return nodeData{}
}

func (m *mockCluster) Stash(ctx context.Context, ri imageSpecifier, sizeHints string, replication int, minReplication int, backend Backend) []string {
	if m.StashFunc != nil {
		return m.StashFunc(ctx, ri, sizeHints, replication, minReplication, backend)
	}
	return nil
}

func (m *mockCluster) Uploaded(r imageRecord) {
	if m.UploadedFunc != nil {
		m.UploadedFunc(r)
	}
}

func (m *mockCluster) GetNeighbors() []nodeData {
	if m.GetNeighborsFunc != nil {
		return m.GetNeighborsFunc()
	}
	return nil
}

func (m *mockCluster) FindNeighborByUUID(uuid string) (*nodeData, bool) {
	if m.FindNeighborByUUIDFunc != nil {
		return m.FindNeighborByUUIDFunc(uuid)
	}
	return nil, false
}

func (m *mockCluster) UpdateNeighbor(n nodeData) {
	if m.UpdateNeighborFunc != nil {
		m.UpdateNeighborFunc(n)
	}
}

func (m *mockCluster) AddNeighbor(n nodeData) {
	if m.AddNeighborFunc != nil {
		m.AddNeighborFunc(n)
	}
}

func (m *mockCluster) Stashed(r imageRecord) {
	if m.StashedFunc != nil {
		m.StashedFunc(r)
	}
}

func (m *mockCluster) GetRecentlyVerified() []imageRecord {
	if m.GetRecentlyVerifiedFunc != nil {
		return m.GetRecentlyVerifiedFunc()
	}
	return nil
}

func (m *mockCluster) GetRecentlyUploaded() []imageRecord {
	if m.GetRecentlyUploadedFunc != nil {
		return m.GetRecentlyUploadedFunc()
	}
	return nil
}

func (m *mockCluster) GetRecentlyStashed() []imageRecord {
	if m.GetRecentlyStashedFunc != nil {
		return m.GetRecentlyStashedFunc()
	}
	return nil
}

func TestImageView_GetImage_serveDirect(t *testing.T) {
	// Mock Backend
	backend := &mockBackend{
		ReadFunc: func(spec imageSpecifier) ([]byte, error) {
			return []byte("image data"), nil
		},
	}

	// Create ImageView with mocks
	imageView := NewImageView(nil, backend, &siteConfig{}, sharedChannels{}, log.NewNopLogger())

	// Test case
	hash, _ := hashFromString("c1986af3c26609b8b7d8933f99c51c1a89e9ea6b", "")
	ri := &imageSpecifier{Hash: hash, Size: resize.MakeSizeSpec("100s"), Extension: ".png"}

	// Execute
	imgData, _, err := imageView.GetImage(context.Background(), ri)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if string(imgData) != "image data" {
		t.Errorf("Expected image data to be 'image data', but got '%s'", string(imgData))
	}
}

func TestImageView_GetImage_serveFromCluster(t *testing.T) {
	// Mock Backend and Cluster
	backend := &mockBackend{
		ReadFunc: func(spec imageSpecifier) ([]byte, error) {
			return nil, errors.New("not found")
		},
	}
	cluster := &mockCluster{
		RetrieveImageFunc: func(ctx context.Context, ri *imageSpecifier) ([]byte, error) {
			return []byte("cluster image data"), nil
		},
	}

	// Create ImageView with mocks
	imageView := NewImageView(cluster, backend, &siteConfig{}, sharedChannels{}, log.NewNopLogger())

	// Test case
	hash, _ := hashFromString("c1986af3c26609b8b7d8933f99c51c1a89e9ea6b", "")
	ri := &imageSpecifier{Hash: hash, Size: resize.MakeSizeSpec("100s"), Extension: ".png"}

	// Execute
	imgData, _, err := imageView.GetImage(context.Background(), ri)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if string(imgData) != "cluster image data" {
		t.Errorf("Expected image data to be 'cluster image data', but got '%s'", string(imgData))
	}
}

func TestImageView_GetImage_serveScaledFromCluster(t *testing.T) {
	// Mock Backend and Cluster
	backend := &mockBackend{
		ReadFunc: func(spec imageSpecifier) ([]byte, error) {
			// Full size is available, but scaled is not
			if spec.Size.IsFull() {
				return []byte("full size image data"), nil
			}
			return nil, errors.New("not found")
		},
	}
	cluster := &mockCluster{
		GetMyselfFunc: func() nodeData {
			return nodeData{Writeable: false} // Not writeable
		},
		RetrieveImageFunc: func(ctx context.Context, ri *imageSpecifier) ([]byte, error) {
			return []byte("scaled cluster image data"), nil
		},
	}

	// Create ImageView with mocks
	imageView := NewImageView(cluster, backend, &siteConfig{}, sharedChannels{}, log.NewNopLogger())

	// Test case
	hash, _ := hashFromString("c1986af3c26609b8b7d8933f99c51c1a89e9ea6b", "")
	ri := &imageSpecifier{Hash: hash, Size: resize.MakeSizeSpec("100s"), Extension: ".png"}

	// Execute
	imgData, _, err := imageView.GetImage(context.Background(), ri)

	// Assert
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if string(imgData) != "scaled cluster image data" {
		t.Errorf("Expected image data to be 'scaled cluster image data', but got '%s'", string(imgData))
	}
}