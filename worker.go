package main

import (
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/thraxil/resize"

	"github.com/go-kit/log"
	"github.com/h2non/bimg"
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
}

func resizeWorker(requests chan resizeRequest, sl log.Logger, s *siteConfig) {
	for req := range requests {
		if !s.Writeable {
			// node is not writeable, so we should never handle a resize
			req.Response <- resizeResponse{nil, false}
			continue
		}
		_ = sl.Log("level", "INFO", "msg", "handling a resize request", "path", req.Path)
		t0 := time.Now()
		fi, err := os.Stat(req.Path)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "resize worker couldn't stat path",
				"path", req.Path, "error", err)
			req.Response <- resizeResponse{nil, false}
			continue
		}
		if fi.IsDir() {
			_ = sl.Log("level", "ERR", "msg", "can't resize a directory",
				"path", req.Path)
			req.Response <- resizeResponse{nil, false}
			continue
		}
		origFile, err := os.Open(req.Path)
		if err != nil {
			_ = origFile.Close()
			_ = sl.Log("level", "ERR", "msg", "resize worker could not open image",
				"image", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, false}
			continue
		} else {
			_ = origFile.Close()
		}
		// Use bimg for image processing
		imageBuffer, err := ioutil.ReadFile(req.Path)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not read image file for bimg", "path", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, false}
			continue
		}

		bimgImage := bimg.NewImage(imageBuffer)
		_, err = bimgImage.Size()
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not get image size for bimg", "path", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, false}
			continue
		}

		sSpec := resize.MakeSizeSpec(req.Size)
		options := bimg.Options{
			Quality: 95,
			NoAutoRotate: false, // Let bimg handle auto-orientation
		}

		if sSpec.IsSquare() {
			options.Width = sSpec.Width()
			options.Height = sSpec.Height()
			options.Crop = true
			options.Gravity = bimg.GravityCentre
		} else {
			options.Width = sSpec.Width()
			options.Height = sSpec.Height()
		}

		newImage, err := bimgImage.Process(options)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "bimg processing failed", "path", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, false}
			continue
		}

		outputPath := resizedPath(req.Path, req.Size)
		err = ioutil.WriteFile(outputPath, newImage, 0644)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not write processed image file", "path", outputPath, "error", err.Error())
			req.Response <- resizeResponse{nil, false}
			continue
		}

		_ = sl.Log("level", "INFO", "msg", "successfully resized image with bimg")
		req.Response <- resizeResponse{nil, true}
		t1 := time.Now()
		_ = sl.Log("level", "INFO", "msg", "finished resize", "time", t1.Sub(t0))

	}
}


func resizedPath(path, size string) string {
	d := filepath.Dir(path)
	extension := filepath.Ext(path)
	return d + "/" + size + extension
}
