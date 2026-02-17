package main

import (
	"image"
	"io"
)

type mockBackend struct {
	fullPathFunc func(ri imageSpecifier) string
}

func (m mockBackend) String() string {
	return "mockBackend"
}

func (m mockBackend) WriteSized(ri imageSpecifier, f io.ReadCloser) error {
	return nil
}

func (m mockBackend) WriteFull(ri imageSpecifier, f io.ReadCloser) error {
	return nil
}

func (m mockBackend) Read(ri imageSpecifier) ([]byte, error) {
	return nil, nil
}

func (m mockBackend) Exists(ri imageSpecifier) bool {
	return false
}

func (m mockBackend) Delete(ri imageSpecifier) error {
	return nil
}

func (m mockBackend) writeLocalType(ri imageSpecifier, i image.Image, e encfunc) {}

func (m mockBackend) fullPath(ri imageSpecifier) string {
	if m.fullPathFunc != nil {
		return m.fullPathFunc(ri)
	}
	return ""
}

func makeNewClusterData(neighbors []nodeData) (nodeData, *cluster) {
	myself := nodeData{
		Nickname:  "myself",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}

	c := newCluster(myself)
	for _, n := range neighbors {
		c.AddNeighbor(n)
	}
	return myself, c
}
