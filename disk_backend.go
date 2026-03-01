package main

import (
	"image"
	"io"

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

func (d diskBackend) WriteSized(img imageSpecifier, r io.ReadCloser) (err error) {
	path := img.baseDir(d.Root)

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	fullpath := img.sizedPath(d.Root)
	f, err := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return f.Sync()
}

func (d diskBackend) WriteFull(img imageSpecifier, r io.ReadCloser) (err error) {
	path := img.baseDir(d.Root)

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	fullpath := img.fullSizePath(d.Root)
	f, err := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return f.Sync()
}

func (d diskBackend) Read(img imageSpecifier) ([]byte, error) {
	path := img.sizedPath(d.Root)
	return os.ReadFile(path)
}

func (d diskBackend) Exists(img imageSpecifier) bool {
	path := img.sizedPath(d.Root)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (d diskBackend) Delete(img imageSpecifier) error {
	path := img.sizedPath(d.Root)
	return os.RemoveAll(path)
}

func (d diskBackend) writeLocalType(ri imageSpecifier, outputImage image.Image, encFunc encfunc) error {
	wFile, err := os.OpenFile(ri.sizedPath(d.Root), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func() {
		_ = wFile.Close()
	}()
	if err := encFunc(wFile, outputImage); err != nil {
		return err
	}
	return wFile.Sync()
}

func (d diskBackend) fullPath(ri imageSpecifier) string {
	return ri.fullSizePath(d.Root)
}
