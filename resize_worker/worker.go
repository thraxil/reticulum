package resize_worker

import (
	"fmt"
	"github.com/thraxil/resize"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"log/syslog"
	"os"
	//  "../../resize"
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
	Success bool
}

var decoders = map[string](func(io.Reader) (image.Image, error)){
	"jpg": jpeg.Decode,
	"gif": gif.Decode,
	"png": png.Decode,
}

func ResizeWorker(requests chan ResizeRequest) {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum.resize-worker")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	for req := range requests {
		sl.Info("handling a resize request")
		t0 := time.Now()
		origFile, err := os.Open(req.Path)
		defer origFile.Close()
		if err != nil {
			sl.Err(fmt.Sprintf("resize worker could not open %s: %s", req.Path, err.Error()))
			req.Response <- ResizeResponse{nil,false}
			continue
		}
		m, err := decoders[req.Extension[1:]](origFile)
		if err != nil {
			sl.Err(fmt.Sprintf("could not find an appropriate decoder for %s (%s): %s",req.Path, req.Extension, err.Error()))
			req.Response <- ResizeResponse{nil,false}
			continue
		}
		outputImage := resize.Resize(m, req.Size)
		// send our response
		req.Response <- ResizeResponse{&outputImage,true}
		t1 := time.Now()
		sl.Info(fmt.Sprintf("finished resize [%v]", t1.Sub(t0)))
	}
}
