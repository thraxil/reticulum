package main

import (
	"testing"
)

func Test_hashToPath(t *testing.T) {
}

type hstp_testcase struct {
	Input  string
	Output string
}

func Test_hashStringToPath(t *testing.T) {
	var testcases = []hstp_testcase{
		hstp_testcase{
			"30de73dcec0ab2de54035edda643ada69dcd60c4",
			"30/de/73/dc/ec/0a/b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60/c4",
		},
	}
	for _, tc := range testcases {
		if tc.Output != hashStringToPath(tc.Input) {
			t.Error("bad path from hash string")
		}
	}
}
