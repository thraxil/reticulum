package main

import (
	"github.com/HugoSmits86/nativewebp"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

var jpegOptions = jpeg.Options{Quality: 90}
var gifOptions = gif.Options{}

type encfunc func(io.Writer, image.Image) error

func encodeJPEG(out io.Writer, in image.Image) error {
	return jpeg.Encode(out, in, &jpegOptions)
}

func encodeGIF(out io.Writer, in image.Image) error {
	return gif.Encode(out, in, &gifOptions)
}

func encodeWebP(out io.Writer, in image.Image) error {
	return nativewebp.Encode(out, in, nil)
}

func encodePNG(out io.Writer, in image.Image) error {
	return png.Encode(out, in)
}

var extencoders = map[string]encfunc{
	".jpg":  encodeJPEG,
	".gif":  encodeGIF,
	".png":  encodePNG,
	".webp": encodeWebP,
}
