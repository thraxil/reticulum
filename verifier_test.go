package main

import (
	"context"
	"fmt"
	"testing"
)

func Test_hashStringFromPath(t *testing.T) {
	var path = "30/de/73/dc/ec/0a/b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60/c4/file.jpg"
	var h = "30de73dcec0ab2de54035edda643ada69dcd60c4"
	o, err := hashStringFromPath(path)
	if err != nil || o != h {
		t.Error("broked!")
	}
	// test the error conditions
	// 1: not enough parts
	path = "b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60/c4/file.jpg"
	o2, err2 := hashStringFromPath(path)
	if err2 == nil || o2 != "" {
		t.Error("error not handled")
	}
	// 2: invalid parts
	path = "30/de/73/dc/ec/0a/b2/de/54/03/5e/dd/a6/43/ad/a6/9d/cd/60blah/c4/aa/aa/aa/foo.jpg"
	o3, err3 := hashStringFromPath(path)
	if err3 == nil || o3 != "" {
		fmt.Println(err3)
		fmt.Println(o3)
		t.Error("error not handled")
	}
}

func Test_basename(t *testing.T) {
	if basename("foo.jpg") != "foo" {
		t.Error("basename failed on simplest case")
	}
	if basename("/foo/bar/baz.jpg") != "baz" {
		t.Error("basename(foo/bar/baz.jpg)")
	}
}

type fdummy struct {
	DirValue  bool
	NameValue string
}

func (f fdummy) IsDir() bool  { return f.DirValue }
func (f fdummy) Name() string { return f.NameValue }

func Test_clearCachedFile(t *testing.T) {
	r := func(p string) error { return nil }
	if clearCachedFile(fdummy{DirValue: true}, "foo", ".jpg", r) != nil {
		t.Error("clearCachedFile() should not have returned non-nil")
	}
	if clearCachedFile(fdummy{DirValue: false, NameValue: "full.jpg"}, "foo", ".jpg", r) != nil {
		t.Error("clearCachedFile() should not have returned non-nil")
	}
	if clearCachedFile(fdummy{DirValue: false}, "foo", ".jpg", r) != nil {
		t.Error("clearCachedFile() should not have returned non-nil")
	}

}

// dummy out a stashableNode
type sdummy struct {
	StashValue bool
}

func (s *sdummy) Stash(context.Context, imageSpecifier, string, backend) bool { return false }
func (s *sdummy) RetrieveImageInfo(context.Context, *imageSpecifier) (*imageInfoResponse, error) {
	return nil, nil
}

func Test_RetrieveReplica(t *testing.T) {
	sl := newDummyLogger()
	n := sdummy{}
	hash, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
	if err != nil {
		t.Error("bad hash")
	}
	var cn []nodeData
	_, c := makeNewClusterData(cn)
	s := siteConfig{}
	r := newImageRebalancer("foo", ".jpg", hash, c, s, sl)
	result := r.retrieveReplica(&n, true)
	if result != 0 {
		t.Error("satisfied == true should mean no retrieval")
	}
	result = r.retrieveReplica(&n, false)
	if result != 0 {
		t.Error("not satisfied but couldn't stash failed")
	}
}

func Test_visitPreChecks(t *testing.T) {
	sl := newDummyLogger()
	var cn []nodeData
	_, c := makeNewClusterData(cn)
	done, err := visitPreChecks("full.jpg", fdummy{}, nil, c, sl)
	if done {
		t.Errorf("shouldn't have been any problems there: %s", err)
	}
	done, _ = visitPreChecks("foo.jpg", fdummy{}, nil, c, sl)
	if !done {
		t.Error("not 'full.jpg', should not allow")
	}
	done, _ = visitPreChecks("full.jpg", fdummy{DirValue: true}, nil, c, sl)
	if !done {
		t.Error("claims to be a directory")
	}
	done, _ = visitPreChecks("full.jpg", fdummy{}, nil, nil, sl)
	if !done {
		t.Error("nil cluster")
	}

}
