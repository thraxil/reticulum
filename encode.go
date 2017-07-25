package main

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

var jpeg_options = jpeg.Options{Quality: 90}
var gif_options = gif.Options{}

func jpgencode(out io.Writer, in image.Image) error {
	return jpeg.Encode(out, in, &jpeg_options)
}

func gifencode(out io.Writer, in image.Image) error {
	return gif.Encode(out, in, &gif_options)
}

var extencoders = map[string]encfunc{
	".jpg": jpgencode,
	".png": png.Encode,
	".gif": gifencode,
}
