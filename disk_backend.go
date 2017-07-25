package main

import (
	"io"
	"io/ioutil"
	"os"
)

type diskBackend struct {
	Root string
}

func newDiskBackend(root string) diskBackend {
	return diskBackend{Root: root}
}

func (d diskBackend) String() string {
	return "Disk"
}

func (d diskBackend) Write(img ImageSpecifier, r io.ReadCloser) error {
	path := img.baseDir(d.Root)

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	fullpath := img.sizedPath(d.Root)
	f, err := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (d diskBackend) Read(img *ImageSpecifier) ([]byte, error) {
	path := img.sizedPath(d.Root)
	return ioutil.ReadFile(path)
}

func (d diskBackend) Exists(img ImageSpecifier) bool {
	path := img.sizedPath(d.Root)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (d diskBackend) Delete(img ImageSpecifier) error {
	path := img.sizedPath(d.Root)
	return os.RemoveAll(path)
}
