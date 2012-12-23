package main

import (
	"fmt"
	"testing"
)

func Test_HashFromString(t *testing.T) {
	h, err := HashFromString("ae28605f0ffc34fe5314342f78efaa13ee45f699", "")
	if err != nil {
		t.Error("bad hash")
	}
	if h.String() != "ae28605f0ffc34fe5314342f78efaa13ee45f699" {
		t.Error("doesn't match")
	}
	if h.Algorithm != "sha1" {
		t.Error("wrong algorithm")
	}
	if h.AsPath() != "ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99" {
		t.Error(fmt.Sprintf("wrong path: %s", h.AsPath()))
	}
}
