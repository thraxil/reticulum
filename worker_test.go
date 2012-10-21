package main

import (
	_ "fmt"
	"testing"
)

type rptestcase struct {
	Path   string
	Size   string
	Output string
}

func Test_resizedPath(t *testing.T) {
	var testCases = []rptestcase{
		rptestcase{"/foo/bar/image.jpg", "100w", "/foo/bar/100w.jpg"},
		rptestcase{"image.jpg", "200w", "./200w.jpg"},
		// this is current behavior, but should it fail instead?
		rptestcase{"/foo/bar/image.jpg", "", "/foo/bar/.jpg"},
	}
	for _, tc := range testCases {
		if tc.Output != resizedPath(tc.Path, tc.Size) {
			t.Error("incorrect output")
		}
	}
}

func Test_convertArgs(t *testing.T) {

}
