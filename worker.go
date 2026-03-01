package main

import (
	"image"
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
	OutputData  []byte
	Success     bool
}

func resizeWorker(requests chan resizeRequest, sl log.Logger, s *siteConfig) {
	for req := range requests {
		if !s.Writeable {
			// node is not writeable, so we should never handle a resize
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}
		_ = sl.Log("level", "INFO", "msg", "handling a resize request", "path", req.Path)
		t0 := time.Now()
		fi, err := os.Stat(req.Path)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "resize worker couldn't stat path",
				"path", req.Path, "error", err)
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}
		if fi.IsDir() {
			_ = sl.Log("level", "ERR", "msg", "can't resize a directory",
				"path", req.Path)
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}
		origFile, err := os.Open(req.Path)
		if err != nil {
			_ = origFile.Close()
			_ = sl.Log("level", "ERR", "msg", "resize worker could not open image",
				"image", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		} else {
			_ = origFile.Close()
		}
		// Use bimg for image processing
		imageBuffer, err := os.ReadFile(req.Path)
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not read image file for bimg", "path", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		bimgImage := bimg.NewImage(imageBuffer)
		_, err = bimgImage.Size()
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not get image size for bimg", "path", req.Path, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		sSpec := resize.MakeSizeSpec(req.Size)
		options := bimg.Options{
			Quality:      95,
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
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		outputPath := resizedPath(req.Path, req.Size)
		// Write to a temporary file first
		tmpFile, err := os.CreateTemp(filepath.Dir(outputPath), "resize-*.tmp")
		if err != nil {
			_ = sl.Log("level", "ERR", "msg", "could not create temp file", "path", outputPath, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}
		tmpName := tmpFile.Name()

		if _, err := tmpFile.Write(newImage); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
			_ = sl.Log("level", "ERR", "msg", "could not write to temp file", "path", tmpName, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpName)
			_ = sl.Log("level", "ERR", "msg", "could not close temp file", "path", tmpName, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		if err := os.Chmod(tmpName, 0644); err != nil {
			_ = os.Remove(tmpName)
			_ = sl.Log("level", "ERR", "msg", "could not chmod temp file", "path", tmpName, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		if err := os.Rename(tmpName, outputPath); err != nil {
			_ = os.Remove(tmpName)
			_ = sl.Log("level", "ERR", "msg", "could not rename temp file to output path", "tmp", tmpName, "output", outputPath, "error", err.Error())
			req.Response <- resizeResponse{nil, nil, false}
			continue
		}

		_ = sl.Log("level", "INFO", "msg", "successfully resized image with bimg")
		req.Response <- resizeResponse{nil, newImage, true}
		t1 := time.Now()
		_ = sl.Log("level", "INFO", "msg", "finished resize", "time", t1.Sub(t0))

	}
}

func resizedPath(path, size string) string {
	d := filepath.Dir(path)
	extension := filepath.Ext(path)
	return d + "/" + size + extension
}
