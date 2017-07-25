package main

import (
	"strings"

	"github.com/thraxil/resize"
)

// combination of field that uniquely specify an image
type ImageSpecifier struct {
	Hash      *Hash
	Size      *resize.SizeSpec
	Extension string
}

func (i ImageSpecifier) String() string {
	return i.Hash.String() + "/" + i.Size.String() + "/image" + i.Extension
}

func NewImageSpecifier(s string) *ImageSpecifier {
	parts := strings.Split(s, "/")
	ahash, _ := HashFromString(parts[0], "")
	size := parts[1]
	rs := resize.MakeSizeSpec(size)
	filename := parts[2]
	fparts := strings.Split(filename, ".")
	extension := "." + fparts[1]
	return &ImageSpecifier{Hash: ahash, Size: rs, Extension: extension}
}

func (i ImageSpecifier) sizedPath(upload_dir string) string {
	return resizedPath(i.fullSizePath(upload_dir), i.Size.String())
}

func (i ImageSpecifier) baseDir(upload_dir string) string {
	return upload_dir + i.Hash.AsPath()
}

func (i ImageSpecifier) fullSizePath(upload_dir string) string {
	return i.baseDir(upload_dir) + "/full" + i.Extension
}

func (i ImageSpecifier) retrieveUrlPath() string {
	ext := strings.TrimLeft(i.Extension, ".")
	return "/retrieve/" + i.Hash.String() + "/" + i.Size.String() + "/" + ext + "/"
}

func (i ImageSpecifier) retrieveInfoUrlPath() string {
	return "/retrieve_info/" + i.Hash.String() + "/" + i.Size.String() + "/" + i.Extension + "/"
}

func (i ImageSpecifier) fullVersion() ImageSpecifier {
	i.Size = resize.MakeSizeSpec("full")
	return i
}
