package main

import (
	"fmt"
	"image"
	"io"
)

type backend interface {
	fmt.Stringer
	WriteSized(imageSpecifier, io.ReadCloser) error
	WriteFull(imageSpecifier, io.ReadCloser) error
	Read(imageSpecifier) ([]byte, error)
	Exists(imageSpecifier) bool
	Delete(imageSpecifier) error
	writeLocalType(imageSpecifier, image.Image, encfunc)
	fullPath(imageSpecifier) string
}
