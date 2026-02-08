package main

import (
	"testing"
)

func Test_hashFromString(t *testing.T) {
	h, err := hashFromString("ae28605f0ffc34fe5314342f78efaa13ee45f699", "")
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
		t.Errorf("wrong path: %s", h.AsPath())
	}
	_, err = hashFromString("ae28605f0ffc34e5314342f78efaa13ee45f699", "")
	if err == nil {
		t.Error("non 40 char hash should've been an error")
	}
}

func Test_Valid(t *testing.T) {
	h, _ := hashFromString("ae28605f0ffc34fe5314342f78efaa13ee45f699", "")
	if !h.Valid() {
		t.Error("hash should be valid")
	}
	h.Algorithm = "foo"
	if h.Valid() {
		t.Error("hash should not be valid (only sha1 supported for now)")
	}
}

func Test_hashFromPath(t *testing.T) {
	_, err := hashFromPath("ae/28/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/foo.jpg")
	if err != nil {
		t.Error("bad hash")
	}

	_, err = hashFromPath("ae/28/60/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/foo.jpg")
	if err == nil {
		t.Error("shouldn't be enough pieces")
	}

	_, err = hashFromPath("ae/288/60/5f/0f/fc/34/fe/53/14/34/2f/78/ef/aa/13/ee/45/f6/99/foo.jpg")
	if err == nil {
		t.Error("not 40 chars")
	}

}
