resize.go
=========

Fork of the image resizing library for Go that used to be in Gorilla
(at
http://code.google.com/p/gorilla/source/browse/lib/appengine/example/moustachio/resize/resize.go?r=3dbce6e267e9d497dffbce31220a059f02c4e99d)

but seems to have been taken out. 

What I've done:
--------------

* updated it to work with Go 1.0's image API

What I plan on doing:
---------------------

* improving the quality of the resizing by adding antialiasing or bicubic interpolation
* making the API a little nicer for some common cases. ie, more like Python's PIL.
* experimenting with ways of resizing large images with constrained memory (ie, not ever keeping an entire massive bitmap
in memory all at once).

I may end up dumping this code entirely and just implementing image
resizing by directly porting the relevant parts of Python's PIL to
Go. I haven't really decided yet.

Ultimately, I'm looking to create a library that will give me, for Go,
when I'm used to from Python in terms of resizing/cropping images for
the web. What I'm going for is an image handling foundation for Go
that will let me port my Apomixis distributed image server
(https://github.com/thraxil/apomixis) from Python/Django to Go.

License remains BSD.

Installation
------------

    $ go get github.com/thraxil/resize

Quick Usage Example
-------------------

    package main
    import (
    	"os"
    	"fmt"
    	"image/jpeg"
    	"log"
    	"github.com/thraxil/resize"
    )
    
    
    func main() {
    	file, err := os.Open("test.jpg")
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
    	bounds := m.Bounds()
    	outputImage := resize.Resize(m,bounds,100,100)
    	outBounds := outputImage.Bounds()
    	fmt.Printf("%q\n",outBounds)
      fl, err := os.OpenFile("out.jpg", os.O_CREATE|os.O_RDWR,0644) 
      if err != nil { 
        fmt.Println("couldn't write", err) 
        return 
      } 
      defer fl.Close() 
    	jpeg.Encode(fl, outputImage, nil)
    }

[![Build Status](https://travis-ci.org/thraxil/resize.png)](https://travis-ci.org/thraxil/resize)
