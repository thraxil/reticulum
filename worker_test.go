package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/h2non/bimg"
)

func createTestImage(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := 0; x < 100; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return jpeg.Encode(f, img, nil)
}

type rptestcase struct {
	Path   string
	Size   string
	Output string
}

func Test_resizedPath(t *testing.T) {
	var testCases = []rptestcase{
		{"/foo/bar/image.jpg", "100w", "/foo/bar/100w.jpg"},
		{"image.jpg", "200w", "./200w.jpg"},
		// this is current behavior, but should it fail instead?
		{"/foo/bar/image.jpg", "", "/foo/bar/.jpg"},
	}
	for _, tc := range testCases {
		if tc.Output != resizedPath(tc.Path, tc.Size) {
			t.Error("incorrect output")
		}
	}
}

func TestResizeWorker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "reticulum-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testImagePath := filepath.Join(tmpDir, "test.jpg")
	err = createTestImage(testImagePath)
	if err != nil {
		t.Fatal(err)
	}

	siteConfig := &siteConfig{
		Writeable: true,
	}

	requests := make(chan resizeRequest)
	sl := log.NewNopLogger()

	go resizeWorker(requests, sl, siteConfig)

	responseChan := make(chan resizeResponse)
	req := resizeRequest{
		Path:      testImagePath,
		Extension: ".jpg",
		Size:      "50w",
		Response:  responseChan,
	}

	requests <- req
	response := <-responseChan

	if !response.Success {
		t.Error("Resize was not successful")
	}

	resizedPath := resizedPath(testImagePath, "50w")
	buffer, err := os.ReadFile(resizedPath)
	if err != nil {
		t.Fatalf("could not read resized image: %v", err)
	}

	img := bimg.NewImage(buffer)
	size, err := img.Size()
	if err != nil {
		t.Fatalf("could not get size of resized image: %v", err)
	}

	if size.Width != 50 {
		t.Errorf("Expected width 50, got %d", size.Width)
	}
}
