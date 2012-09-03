package resize_worker

import (
	"fmt"
	"github.com/thraxil/resize"
	"github.com/thraxil/reticulum/config"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log/syslog"
	"os"
	"path/filepath"
	"time"
)

type ResizeRequest struct {
	Path      string
	Extension string
	Size      string
	Response  chan ResizeResponse
}

type ResizeResponse struct {
	OutputImage *image.Image
	Success     bool
	Magick      bool
}

var decoders = map[string](func(io.Reader) (image.Image, error)){
	"jpg": jpeg.Decode,
	"gif": gif.Decode,
	"png": png.Decode,
}

func ResizeWorker(requests chan ResizeRequest, sl *syslog.Writer, s *config.SiteConfig) {
	for req := range requests {
		sl.Info("handling a resize request")
		t0 := time.Now()
		origFile, err := os.Open(req.Path)
		if err != nil {
			origFile.Close()
			sl.Err(fmt.Sprintf("resize worker could not open %s: %s", req.Path, err.Error()))
			req.Response <- ResizeResponse{nil, false, false}
			continue
		}
		_, err = imageMagickResize(req.Path, req.Size, sl, s)
		if err != nil {
			// imagemagick couldn't handle it either
			sl.Err(fmt.Sprintf("imagemagick couldn't handle it: %s", err.Error()))
			req.Response <- ResizeResponse{nil, false, false}
		} else {
			// imagemagick saved the day
			sl.Info("rescued by imagemagick")
			req.Response <- ResizeResponse{nil, true, true}
			t1 := time.Now()
			sl.Info(fmt.Sprintf("finished resize [%v]", t1.Sub(t0)))
		}
	}
}

// Go's built-in image/jpeg can't load progressive jpegs
// so sometimes we need to bail and have imagemagick do the work
// this sucks, is redundant, and i'd rather not have this external dependency
// so this will be removed as soon as Go can handle it all itself
func imageMagickResize(path, size string, sl *syslog.Writer,
	s *config.SiteConfig) (string, error) {

	args := convertArgs(size, path, s)

	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	p, err := os.StartProcess(args[0], args, &os.ProcAttr{Files: fds})
	defer p.Release()
	if err != nil {
		sl.Err("imagemagick failed to start")
		sl.Err(err.Error())
		return "", err
	}
	_, err = p.Wait()
	if err != nil {
		sl.Err("imagemagick failed")
		sl.Err(err.Error())
		return "", err
	}
	return resizedPath(path, size), nil
}

func resizedPath(path, size string) string {
	d := filepath.Dir(path)
	extension := filepath.Ext(path)
	return d + "/" + size + extension
}

func convertArgs(size, path string, c *config.SiteConfig) []string {
	convertBin := c.ImageMagickConvertPath
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
