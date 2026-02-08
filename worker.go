package main

import (
	"fmt"
	"image"

	"os"
	"path/filepath"
	"time"

	"github.com/thraxil/resize"

	"github.com/go-kit/log"
)

type resizeRequest struct {
	Path      string
	Extension string
	Size      string
	Response  chan resizeResponse
}

type resizeResponse struct {
	OutputImage *image.Image
	Success     bool
	Magick      bool
}

func resizeWorker(requests chan resizeRequest, sl log.Logger, s *siteConfig) {
	for req := range requests {
		if !s.Writeable {
			// node is not writeable, so we should never handle a resize
			req.Response <- resizeResponse{nil, false, false}
			continue
		}
		_ = sl.Log("level", "INFO", "msg", "handling a resize request", "path", req.Path)
		t0 := time.Now()
		fi, err := os.Stat(req.Path)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "resize worker couldn't stat path",
				"path", req.Path, "error", err)
			req.Response <- resizeResponse{nil, false, false}
			continue
		}
		if fi.IsDir() {
			_ = sl.Log("level", "ERR", "msg", "can't resize a directory",
				"path", req.Path)
			req.Response <- resizeResponse{nil, false, false}
			continue
		}
		origFile, err := os.Open(req.Path)
		if err != nil {
			_ = origFile.Close()
			_ = sl.Log("level", "ERR", "msg", "resize worker could not open image",
				"image", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, false, false}
			continue
		} else {
			_ = origFile.Close()
		}
		_, err = imageMagickResize(req.Path, req.Size, sl, s)
		if err != nil {
			// imagemagick couldn't handle it either
			_ = sl.Log("level", "ERR", "msg", "imagemagick couldn't handle it", "error", err.Error())
			req.Response <- resizeResponse{nil, false, false}
		} else {
			// imagemagick saved the day
			_ = sl.Log("level", "INFO", "msg", "rescued by imagemagick")
			req.Response <- resizeResponse{nil, true, true}
			t1 := time.Now()
			_ = sl.Log("level", "INFO", "msg", "finished resize", "time", t1.Sub(t0))
		}
	}
}

// Go's built-in image/jpeg can't load progressive jpegs
// so sometimes we need to bail and have imagemagick do the work
// this sucks, is redundant, and i'd rather not have this external dependency
// so this will be removed as soon as Go can handle it all itself
func imageMagickResize(path, size string, sl log.Logger,
	s *siteConfig) (string, error) {

	args := convertArgs(size, path, s.ImageMagickConvertPath)

	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, &os.ProcAttr{Files: fds})
	defer func() { _ = p.Release() }()
	if err != nil {
		_ = sl.Log("level", "ERR", "msg", "imagemagick failed to start",
			"error", err.Error())
		return "", err
	}
	_, err = p.Wait()
	if err != nil {
		_ = sl.Log("level", "ERR", "msg", "imagemagick failed",
			"error", err.Error())
		return "", err
	}
	return resizedPath(path, size), nil
}

func resizedPath(path, size string) string {
	d := filepath.Dir(path)
	extension := filepath.Ext(path)
	return d + "/" + size + extension
}

func convertArgs(size, path, convertBin string) []string {
	// need to convert our size spec to what convert expects
	// we can ignore 'full' since that will never trigger
	// a resize_worker request
	s := resize.MakeSizeSpec(size)
	var maxDim int
	if s.Width() > s.Height() {
		maxDim = s.Width()
	} else {
		maxDim = s.Height()
	}

	var args []string
	if s.IsSquare() {
		args = []string{
			convertBin,
			"-resize",
			s.ToImageMagickSpec(),
			"-auto-orient",
			"-gravity",
			"center",
			"-extent",
			fmt.Sprintf("%dx%d", maxDim, maxDim),
			path,
			resizedPath(path, size),
		}
	} else {
		// BUG(thraxil): this auto orients properly
		// but doesn't switch width/height in that case
		// so 90 or 270 degree rotations come out the wrong
		// size
		args = []string{
			convertBin,
			"-auto-orient",
			"-resize",
			s.ToImageMagickSpec(),
			path,
			resizedPath(path, size),
		}
	}
	return args
}
