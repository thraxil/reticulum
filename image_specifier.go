package main

import (
	"strings"

	"github.com/thraxil/resize"
)

// combination of field that uniquely specify an image
type imageSpecifier struct {
	Hash      *hash
	Size      *resize.SizeSpec
	Extension string // with leading '.'
}

func (i imageSpecifier) String() string {
	return i.Hash.String() + "/" + i.Size.String() + "/image" + i.Extension
}

func newImageSpecifier(s string) *imageSpecifier {
	parts := strings.Split(s, "/")
	ahash, _ := hashFromString(parts[0], "")
	size := parts[1]
	rs := resize.MakeSizeSpec(size)
	filename := parts[2]
	fparts := strings.Split(filename, ".")
	extension := "." + fparts[1]
	return &imageSpecifier{Hash: ahash, Size: rs, Extension: extension}
}

func (i imageSpecifier) sizedPath(uploadDir string) string {
	return resizedPath(i.fullSizePath(uploadDir), i.Size.String())
}

func (i imageSpecifier) baseDir(uploadDir string) string {
	return uploadDir + i.Hash.AsPath()
}

func (i imageSpecifier) fullSizePath(uploadDir string) string {
	return i.baseDir(uploadDir) + "/full" + i.Extension
}

func (i imageSpecifier) retrieveURLPath() string {
	ext := strings.TrimLeft(i.Extension, ".")
	return "/retrieve/" + i.Hash.String() + "/" + i.Size.String() + "/" + ext + "/"
}

func (i imageSpecifier) retrieveInfoURLPath() string {
	return "/retrieve_info/" + i.Hash.String() + "/" + i.Size.String() + "/" + i.Extension + "/"
}

func (i imageSpecifier) fullVersion() imageSpecifier {
	i.Size = resize.MakeSizeSpec("full")
	return i
}
