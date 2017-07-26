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

func Test_clear_cached_file(t *testing.T) {
	r := func(p string) error { return nil }
	if clear_cached_file(fdummy{DirValue: true}, "foo", ".jpg", r) != nil {
		t.Error("clear_cached_file() should not have returned non-nil")
	}
	if clear_cached_file(fdummy{DirValue: false, NameValue: "full.jpg"}, "foo", ".jpg", r) != nil {
		t.Error("clear_cached_file() should not have returned non-nil")
	}
	if clear_cached_file(fdummy{DirValue: false}, "foo", ".jpg", r) != nil {
		t.Error("clear_cached_file() should not have returned non-nil")
	}

}

// dummy out a StashableNode
type sdummy struct {
	StashValue bool
}

func (s *sdummy) Stash(ri ImageSpecifier, size_hints string, backend backend) bool { return false }
func (s *sdummy) RetrieveImageInfo(ri *ImageSpecifier) (*ImageInfoResponse, error) {
	return nil, nil
}

func Test_RetrieveReplica(t *testing.T) {
	sl := NewDummyLogger()
	n := sdummy{}
	hash, err := HashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
	if err != nil {
		t.Error("bad hash")
	}
	cn := make([]NodeData, 0)
	_, c := makeNewClusterData(cn)
	s := SiteConfig{}
	r := NewImageRebalancer("foo", ".jpg", hash, c, s, sl)
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
	sl := NewDummyLogger()
	cn := make([]NodeData, 0)
	_, c := makeNewClusterData(cn)
	done, err := visitPreChecks("full.jpg", fdummy{}, nil, c, sl)
	if done {
		t.Error(fmt.Sprintf("shouldn't have been any problems there: %s", err))
	}
	if err != nil {
		t.Error(fmt.Sprintf("shouldn't have been any problems there: %s", err))
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
