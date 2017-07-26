package main

import (
	"testing"
)

func Test_MyNode(t *testing.T) {
	c := configData{}
	n := c.MyNode()
	if n.Writeable {
		t.Error("couldn't make nodeData")
	}
}

func Test_MyConfig(t *testing.T) {
	c := configData{}
	s := c.MyConfig()
	if s.Writeable {
		t.Error("couldn't make siteConfig")
	}
}

func Test_KeyRequired(t *testing.T) {
	s := siteConfig{}
	if s.KeyRequired() {
		t.Error("shouldn't be any keys listed by default")
	}
	s.UploadKeys = append(s.UploadKeys, "foo")
	if !s.KeyRequired() {
		t.Error("now there should be one")
	}

}

func Test_ValidKey(t *testing.T) {
	s := siteConfig{}
	if s.ValidKey("foo") {
		t.Error("key definitely does not exist, should not pass")
	}
	s.UploadKeys = append(s.UploadKeys, "foo")
	if !s.ValidKey("foo") {
		t.Error("key does exist now")
	}
}
