package main

import (
	_ "fmt"
	"testing"
)

func Test_Create(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	if i.Extension != ".jpg" {
		t.Error("wrong extension")
	}
}

func Test_String(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	if i.String() != "112e42f26fce70d268438ac8137d81607499ee10/200s/image.jpg" {
		t.Error("incorrect stringification")
	}
}

func Test_FullSizePath(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	r := i.fullSizePath("")
	if r != "11/2e/42/f2/6f/ce/70/d2/68/43/8a/c8/13/7d/81/60/74/99/ee/10/full.jpg" {
		t.Error("wrong fullSizePath: %s", r)
	}
}

func Test_SizedPath(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	r := i.sizedPath("")
	if r != "11/2e/42/f2/6f/ce/70/d2/68/43/8a/c8/13/7d/81/60/74/99/ee/10/200s.jpg" {
		t.Error("wrong sizedPath: %s", r)
	}
}

func Test_RetrieveUrlPath(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	r := i.retrieveUrlPath()
	if r != "/retrieve/112e42f26fce70d268438ac8137d81607499ee10/200s/jpg/" {
		t.Error("wrong retrieveUrlPath: %s", r)
	}
}

func Test_RetrieveInfoUrlPath(t *testing.T) {
	s := "112e42f26fce70d268438ac8137d81607499ee10/200s/1250.jpg"
	i := NewImageSpecifier(s)
	r := i.retrieveInfoUrlPath()
	if r != "/retrieve_info/112e42f26fce70d268438ac8137d81607499ee10/200s/.jpg/" {
		t.Error("wrong retreiveInfoUrlPath: %s", r)
	}
}
