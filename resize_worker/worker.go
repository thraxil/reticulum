package resize_worker

import (
	"code.google.com/p/graphics-go/graphics"
	"fmt"
	"github.com/thraxil/exifgo"
	"github.com/thraxil/resize"
	"github.com/thraxil/reticulum/config"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log/syslog"
	"math"
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
		if req.Extension[1:] == "jpg" {
			// check the EXIF to see if it needs to be rotated
			exif_data, err := exifgo.Parse_jpeg(origFile)
			if err == nil {
				// if we can't parse EXIF, don't even try to do anything else with it
				for _, t := range exif_data {
					if t.Label == "Orientation of image" {
						v := t.Content.(uint16)
						// TODO: 2, 3, and 4 could be applied post-scale
						// since they don't change dimensions
						if v == 1 {
							sl.Debug("no rotation needed")
						} else if v == 2 {
							// mirror left-right
							sl.Debug("orient 2")
							// TODO
						} else if v == 3 {
							// mirror upside-down
							// aka, rotate 180
							sl.Debug("orient 3")
							src, _, err := image.Decode(origFile)
							if err == nil {
								srcb := src.Bounds()
								dst := image.NewRGBA(image.Rect(0, 0, srcb.Dy(), srcb.Dx()))
								graphics.Rotate(dst, src, &graphics.RotateOptions{math.Pi})
								jpeg.Encode(origFile, dst, nil)
							}
						} else if v == 4 {
							// mirror left-right and upside-down
							// aka mirror left-right and rotate 180
							sl.Debug("orient 4")
							// TODO
						} else if v == 5 {
							// mirror left-right and rotate 270
							sl.Debug("orient 5")
							// TODO
						} else if v == 6 {
							// rotate 270
							sl.Debug("orient 6")
							src, _, err := image.Decode(origFile)
							if err == nil {
								srcb := src.Bounds()
								dst := image.NewRGBA(image.Rect(0, 0, srcb.Dy(), srcb.Dx()))
								graphics.Rotate(dst, src, &graphics.RotateOptions{3.0 * math.Pi / 2.0})
								jpeg.Encode(origFile, dst, nil)
							}
						} else if v == 7 {
							// mirror left-right and rotate 90
							sl.Debug("orient 7")
							// TODO
						} else if v == 8 {
							// rotate 90
							sl.Debug("orient 8")
							src, _, err := image.Decode(origFile)
							if err == nil {
								srcb := src.Bounds()
								dst := image.NewRGBA(image.Rect(0, 0, srcb.Dy(), srcb.Dx()))
								graphics.Rotate(dst, src, &graphics.RotateOptions{math.Pi / 2.0})
								jpeg.Encode(origFile, dst, nil)
							}
						}
					}
				}
			}
		}
		m, err := decoders[req.Extension[1:]](origFile)
		if err != nil {
			origFile.Close()
			sl.Err(fmt.Sprintf("could not find an appropriate decoder for %s (%s): %s", req.Path, req.Extension, err.Error()))
			// try imagemagick
			_, err := imageMagickResize(req.Path, req.Size, sl, s)
			if err != nil {
				// imagemagick couldn't handle it either
				sl.Err(fmt.Sprintf("imagemagick couldn't handle it either: %s", err.Error()))
				req.Response <- ResizeResponse{nil, false, false}
			} else {
				// imagemagick saved the day
				sl.Info("rescued by imagemagick")
				req.Response <- ResizeResponse{nil, true, true}
				t1 := time.Now()
				sl.Info(fmt.Sprintf("finished resize [%v]", t1.Sub(t0)))
			}
			continue
		}
		outputImage := resize.Resize(m, req.Size)
		// send our response
		origFile.Close()
		req.Response <- ResizeResponse{&outputImage, true, false}
		t1 := time.Now()
		sl.Info(fmt.Sprintf("finished resize [%v]", t1.Sub(t0)))
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
			fmt.Sprintf("%dx%d^", maxDim, maxDim),
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
			fmt.Sprintf("%dx%d", maxDim, maxDim),
			path,
			resizedPath(path, size),
		}
	}
	return args
}
