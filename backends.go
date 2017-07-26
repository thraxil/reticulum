package main

import (
	"fmt"
	"image"
	"io"
)

type backend interface {
	fmt.Stringer
	WriteSized(ImageSpecifier, io.ReadCloser) error
	WriteFull(ImageSpecifier, io.ReadCloser) error
	Read(ImageSpecifier) ([]byte, error)
	Exists(ImageSpecifier) bool
	Delete(ImageSpecifier) error
	writeLocalType(ImageSpecifier, image.Image, encfunc)
	fullPath(ImageSpecifier) string
}
