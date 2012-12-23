package main

import (
	"fmt"
	"testing"
)

func Test_hashFromPath(t *testing.T) {
	var path = "30/de/73/dc/ec/0a/b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60/c4/file.jpg"
	var h = "30de73dcec0ab2de54035edda643ada69dcd60c4"
	o, err := hashFromPath(path)
	if err != nil || o != h {
		t.Error("broked!")
	}
	// test the error conditions
	// 1: not enough parts
	path = "b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60/c4/file.jpg"
	o2, err2 := hashFromPath(path)
	if err2 == nil || o2 != "" {
		t.Error("error not handled")
	}
	// 2: invalid parts
	path = "30/de/73/dc/ec/0a/b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60blah/c4/aa/aa/aa/foo.jpg"
	o3, err3 := hashFromPath(path)
	if err3 == nil || o3 != "" {
		fmt.Println(err3)
		fmt.Println(o3)
		t.Error("error not handled")
	}

}
