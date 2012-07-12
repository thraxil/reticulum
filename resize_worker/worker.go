package resize_worker

import (
	"image"
	"image/jpeg"
	"image/png"
	"image/gif"
	"io"
	"os"
	"github.com/thraxil/resize"
	//  "../../resize"

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
	for req := range requests {
		// TODO: filesystem and resize error handlers
		origFile, _ := os.Open(req.Path)
		defer origFile.Close()
		m, _ := decoders[req.Extension[1:]](origFile)
		outputImage := resize.Resize(m, req.Size)
		// send our response
		req.Response <- ResizeResponse{&outputImage}
	}
}
