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
		// TODO: filesystem and resize error handlers
		sl.Info("handling a resize request")
		t0 := time.Now()
		origFile, _ := os.Open(req.Path)
		defer origFile.Close()
		m, _ := decoders[req.Extension[1:]](origFile)
		outputImage := resize.Resize(m, req.Size)
		// send our response
		req.Response <- ResizeResponse{&outputImage}
		t1 := time.Now()
		sl.Info(fmt.Sprintf("finished resize [%v]", t1.Sub(t0)))
	}
}
