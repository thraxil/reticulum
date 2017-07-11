package main

import (
	"../../resize"
	"flag"
	"fmt"
	"image/jpeg"
	"log"
	"os"
)

func main() {
	var source string
	flag.StringVar(&source,"source", "./test.jpg", "image to read")
	var dest string
	flag.StringVar(&dest, "dest", "./out.jpg", "image to output")

	var sizeStr string

	flag.StringVar(&sizeStr, "size", "100w", "size to resize to")

  flag.Parse()
	file, err := os.Open(source)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Decode the image.
	m, err := jpeg.Decode(file)
	if err != nil {
		fmt.Printf("error decoding image\n")
		log.Fatal(err)
	}
	outputImage := resize.Resize(m, sizeStr)
	fl, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("couldn't write", err)
		return
	}
	defer fl.Close()
	jpeg.Encode(fl, outputImage, nil)
}
