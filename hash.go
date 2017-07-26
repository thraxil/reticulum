package main

import (
	_ "bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type hash struct {
	Algorithm string
	Value     []byte
}

func hashFromPath(path string) (*hash, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return nil, errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return nil, fmt.Errorf("invalid hash length: %d (%s)", len(hash), hash)
	}
	return hashFromString(hash, "sha1")
}

func hashFromString(str, algorithm string) (*hash, error) {
	if algorithm == "" {
		algorithm = "sha1"
	}
	if len(str) != 40 {
		return nil, errors.New("invalid hash")
	}
	return &hash{algorithm, []byte(str)}, nil
}

func (h hash) AsPath() string {
	var parts []string
	s := h.String()
	for i := range s {
		if (i % 2) != 0 {
			parts = append(parts, s[i-1:i+1])
		}
	}
	return strings.Join(parts, "/")
}

func (h hash) String() string {
	return string(h.Value)
}

func (h hash) Valid() bool {
	return h.Algorithm == "sha1" && len(h.String()) == 40
}
