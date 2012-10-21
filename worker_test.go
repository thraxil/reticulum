package main

import (
	"fmt"
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

type catestcase struct {
	Size       string
	Path       string
	ConvertBin string
	Output     []string
}

func Test_convertArgs(t *testing.T) {
	var testCases = []catestcase{
		catestcase{"100s", "/foo/bar/image.jpg", "/usr/bin/convert",
			[]string{
				"/usr/bin/convert",
				"-resize",
				"100x100^",
				"-auto-orient",
				"-gravity",
				"center",
				"-extent",
				"100x100",
				"/foo/bar/image.jpg",
				"/foo/bar/100s.jpg",
			},
		},
		catestcase{"100w", "/foo/bar/image.jpg", "/usr/bin/convert",
			[]string{
				"/usr/bin/convert",
				"-auto-orient",
				"-resize",
				"100",
				"/foo/bar/image.jpg",
				"/foo/bar/100w.jpg",
			},
		},
	}
	for _, tc := range testCases {
		output := convertArgs(tc.Size, tc.Path, tc.ConvertBin)
		for i := range output {
			if tc.Output[i] != output[i] {
				fmt.Printf("%s %s\n", tc.Output[i], output[i])
				t.Error("incorrect convert args")
			}
		}
	}
}
