[![Build Status](https://travis-ci.org/thraxil/resize.svg?branch=master)](https://travis-ci.org/thraxil/resize)
[![Coverage Status](https://coveralls.io/repos/github/thraxil/resize/badge.svg?branch=master)](https://coveralls.io/github/thraxil/resize?branch=master)

resize.go
=========

This started out as a fork of the image resizing library for Go that
used to be in Gorilla (I can't even find a link to the original code,
it's that old) but seems to have been taken out.

It has since pretty much entirely been gutted and replaced.

Now, the main functionality that this library provides is a very
opinionated view of image resizing and thumbnailing.

Specifically, it seems that every image resizing library out there
expects you to tell it the width and height of the image that you are
generating. That's fine except that in about two decades of web and
application development, much of which has involved images, I've
*never* encountered a situation where changing the aspect ratio of an
image is desirable (not 100% true: sometimes if you're feeding things
to a computer vision library, that's what you want, but never when
generating images that humans will look at). Every image resizing use
case I've encountered has been basically:

* make the image fit within a particular width *or* height while
  preserving the aspect ratio. Eg, scale it down so it's 100px tall.
* *crop* the image so it is a particular aspect ratio and then scale
  it to a particular size. 90% of the time this is a square.
* on very rare occasions, you need to scale the image to fit within a
  bounding box, while preserving the aspect ratio. Eg, fit it within a
  100px wide by 200px box.

You just never need to change the aspect ratio, squishing and
distorting the image. And yet, every image resizing API seems to lead
you in that direction by default. Coaxing them to do the three use
cases described above is almost always much more work than it ought to
be.

So, that's what this library is for.

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
		outputImage := resize.Resize(m, "100s")
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

