package main

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

var jpegOptions = jpeg.Options{Quality: 90}
var gifOptions = gif.Options{}

type encfunc func(io.Writer, image.Image) error

func jpgencode(out io.Writer, in image.Image) error {
	return jpeg.Encode(out, in, &jpegOptions)
}

func gifencode(out io.Writer, in image.Image) error {
	return gif.Encode(out, in, &gifOptions)
}

var extencoders = map[string]encfunc{
	".jpg": jpgencode,
	".png": png.Encode,
	".gif": gifencode,
}
