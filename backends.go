package main

import (
	"fmt"
	"io"
)

type backend interface {
	fmt.Stringer
	Write(ImageSpecifier, io.ReadCloser) error
	Read(*ImageSpecifier) ([]byte, error)
	Exists(ImageSpecifier) bool
	Delete(ImageSpecifier) error
}
