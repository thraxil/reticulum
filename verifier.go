package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/thraxil/randwalk"
	"github.com/thraxil/resize"
)

// part of the path that's not a directory or extension
func basename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

// convert some/base/12/34/45/56/67/.../file.jpg to 1234455667
// expects a filename at the end. otherwise, there can be
// arbitrarily many extra path components on the front
func hashFromPath(path string) (string, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return "", errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return "", errors.New(fmt.Sprintf("invalid hash length: %d (%s)", len(hash), hash))
	}
	return hash, nil
}

// checks the image for corruption
// if it is corrupt, try to repair
func verify(path string, extension string, hash *Hash, ahash string,
	c *Cluster, sl log.Logger) error {
	//    VERIFY PHASE
	if hash.String() != ahash {
		sl.Log("level", "WARN", "msg", "image appears to be corrupted!", "image", path)
		corruptedImages.Add(1)
		// trust that the hash was correct on upload
		// ask other nodes for a copy
		repaired, err := repair_image(path, extension, hash, c, sl)
		if err != nil {
			sl.Log("level", "ERR", "msg", "error attempting to repair image", "error", err.Error())
			return err
		}
		if repaired {
			repairedImages.Add(1)
			err := clear_cached(path, extension)
			if err != nil {
				return err
			}
		} else {
			sl.Log("level", "ERR", "msg", "could not repair corrupted image", "image", path)
			unrepairableImages.Add(1)
			// return here so we don't try to rebalance a corrupted image
			return errors.New("unrepairable image")
		}
	}
	verifiedImages.Add(1)
	return nil
}

// do our best to repair the image
func repair_image(path string, extension string, hash *Hash,
	c *Cluster, sl log.Logger) (bool, error) {
	nodes_to_check := c.ReadOrder(hash.String())
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {
			// skip ourself, since we know we are corrupt
			continue
		}
		cont, ret, err := checkImageOnNode(n, hash, extension, path, c, sl)
		if !cont {
			return ret, err
		}
	}
	return false, nil
}

func replaceImageWithCorrected(path string, img []byte, sl log.Logger) (bool, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		// can't open for writing!
		sl.Log("level", "ERR", "msg", "could not open for writing", "image", path, "error", err.Error())
		f.Close()
		return false, false, err
	}
	_, err = f.Write(img)
	f.Close()
	if err != nil {
		sl.Log("level", "ERR", "msg", "could not write", "image", path, "error", err.Error())
		return false, false, err
	}
	return false, true, nil
}

func checkImageOnNode(n NodeData, hash *Hash, extension string, path string,
	c *Cluster, sl log.Logger) (bool, bool, error) {
	s := resize.MakeSizeSpec("full")
	ri := &ImageSpecifier{hash, s, extension}

	img, err := n.RetrieveImage(ri)
	if err != nil {
		// doesn't have it
		sl.Log("level", "INFO", "node", n.Nickname,
			"msg", "node does not have a copy of the desired image")
		return true, true, nil
	} else {
		if !doublecheck_replica(img, hash) {
			// the copy from that node isn't right either
			return true, true, nil
		}
		return replaceImageWithCorrected(path, img, sl)
	}
}

func doublecheck_replica(img []byte, hash *Hash) bool {
	hn := sha1.New()
	io.WriteString(hn, string(img))
	nhash := fmt.Sprintf("%x", hn.Sum(nil))
	return nhash == hash.String()
}

// the only File methods that we care about
// makes it easier to mock
type FileIsh interface {
	IsDir() bool
	Name() string
}

// cached sizes may have been created off the broken one
// and the easiest solution is to take off
// and nuke the site from orbit. It's the only way to be sure.
func clear_cached(path string, extension string) error {
	files, err := ioutil.ReadDir(filepath.Dir(path))
	if err != nil {
		// can't read the dir?!
		return err
	}
	var successful_purge = true
	for _, file := range files {
		r := func(p string) error { return os.Remove(p) }
		err = clear_cached_file(file, path, extension, r)
		successful_purge = successful_purge && (err == nil)
	}
	if !successful_purge {
		// one or more cached sizes were not deleted
		return errors.New("could not clear potentially corrupted scaled image")
	}
	return nil
}

type remover func(fullpath string) error

func clear_cached_file(file FileIsh, path, extension string, r remover) error {
	if file.IsDir() {
		return nil
	}
	if file.Name() == "full"+extension {
		return nil
	}
	return r(filepath.Join(filepath.Dir(path), file.Name()))
}

type ImageRebalancer struct {
	c         *Cluster
	s         SiteConfig
	sl        log.Logger
	hash      *Hash
	path      string
	extension string
}

func NewImageRebalancer(path, extension string, hash *Hash, c *Cluster, s SiteConfig, sl log.Logger) *ImageRebalancer {
	return &ImageRebalancer{c, s, sl, hash, path, extension}
}

// check that the image is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func (r ImageRebalancer) Rebalance() error {
	//    REBALANCE PHASE
	if r.c == nil {
		r.sl.Log("level", "ERR", "msg", "rebalance was given a nil cluster")
		return errors.New("nil cluster")
	}
	nodes_to_check := r.c.ReadOrder(r.hash.String())
	satisfied, delete_local, found_replicas := r.checkNodesForRebalance(nodes_to_check)
	if !satisfied {
		r.sl.Log("level", "WARN", "msg", "could not replicate",
			"image", r.path, "replication", r.s.Replication)
		rebalanceFailures.Add(1)
	} else {
		r.sl.Log("level", "INFO", "image", r.path,
			"msg", "full replica set",
			"found_replicas", found_replicas,
			"desired_replicas", r.s.Replication)
		rebalanceSuccesses.Add(1)
	}
	if satisfied && delete_local {
		clean_up_excess_replica(r.path, r.sl)
		rebalanceCleanups.Add(1)
	}
	return nil
}

func (r ImageRebalancer) checkNodesForRebalance(nodes_to_check []NodeData) (bool, bool, int) {
	var satisfied = false
	var found_replicas = 0
	var delete_local = true
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		if n.UUID == r.c.Myself.UUID {
			// don't need to delete it
			delete_local = false
			found_replicas++
		} else {
			found_replicas = found_replicas + r.retrieveReplica(&n, satisfied)
		}
		if found_replicas >= r.s.Replication {
			satisfied = true
		}
		if found_replicas >= r.s.MaxReplication {
			// nothing more to do. other nodes that have excess
			// copies are responsible for deletion. Our job
			// is just to make sure the first N nodes have a copy
			return satisfied, delete_local, found_replicas
		}
	}
	return satisfied, delete_local, found_replicas
}

type StashableNode interface {
	Stash(filename string, size_hints string) bool
	RetrieveImageInfo(ri *ImageSpecifier) (*ImageInfoResponse, error)
}

func (r ImageRebalancer) retrieveReplica(n StashableNode, satisfied bool) int {

	s := resize.MakeSizeSpec("full")
	ri := &ImageSpecifier{r.hash, s, r.extension[1:]}

	img_info, err := n.RetrieveImageInfo(ri)
	if err == nil && img_info != nil && img_info.Local {
		// node should have it. node has it. cool.
		return 1
	} else {
		// that node should have a copy, but doesn't so stash it
		if !satisfied {
			if n.Stash(r.path, "") {
				r.sl.Log("level", "INFO", "msg", "replicated", "image", r.path)
				return 1
			} else {
				// couldn't stash to that node. not writeable perhaps.
				// not really our problem to deal with, but we do want
				// to make sure that another node gets a copy
				// so we don't increment found_replicas
			}
		}
	}
	return 0
}

// our node is not at the front of the list, so
// we have an excess copy. clean that up and make room!
func clean_up_excess_replica(path string, sl log.Logger) {
	err := os.RemoveAll(filepath.Dir(path))
	if err != nil {
		sl.Log("level", "ERR", "msg", "could not clear out excess replica", "image", path,
			"error", err.Error())
	} else {
		sl.Log("level", "INFO", "msg", "cleared excess replica", "image", path)
	}
}

func visitPreChecks(path string, f FileIsh, err error, c *Cluster, sl log.Logger) (bool, error) {
	if err != nil {
		sl.Log("level", "ERR", "msg", "verifier.visit was handed an error", "error", err.Error())
		return true, err
	}
	if c == nil {
		sl.Log("level", "ERR", "msg", "verifier.visit was given a nil cluster")
		return true, errors.New("nil cluster")
	}
	// all we care about is the "full" version of each
	if f.IsDir() {
		return true, nil
	}
	if basename(path) != "full" {
		return true, nil
	}
	return false, nil
}

func visit(path string, f os.FileInfo, err error, c *Cluster,
	s SiteConfig, sl log.Logger) error {
	defer func() {
		if r := recover(); r != nil {
			sl.Log("level", "ERR", "msg", "Error in verifier.visit()", "node", c.Myself.Nickname, "image", path,
				"error", r)
		}
	}()
	done, err := visitPreChecks(path, f, err, c, sl)
	if done {
		return err
	}
	extension := filepath.Ext(path)
	if len(extension) < 2 {
		return nil
	}

	hash, err := HashFromPath(path)
	if err != nil {
		return nil
	}
	h := sha1.New()
	imgfile, err := os.Open(path)
	defer imgfile.Close()
	if err != nil {
		sl.Log("level", "ERR", "msg", "error opening", "image", path, "error", err.Error())
		return err
	}
	_, err = io.Copy(h, imgfile)
	if err != nil {
		sl.Log("level", "ERR", "msg", "error copying", "image", path, "error", err.Error())
		return err
	}
	ahash := fmt.Sprintf("%x", h.Sum(nil))
	err = verify(path, extension, hash, ahash, c, sl)
	if err != nil {
		return err
	}
	r := NewImageRebalancer(path, extension, hash, c, s, sl)
	err = r.Rebalance()
	if err != nil {
		return err
	}
	c.Verified(ImageRecord{*hash, extension})
	// slow things down a little to keep server load down
	var base_time = s.VerifierSleep
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time+jitter) * time.Second)
	return nil
}

// makes a closure that has access to the cluster and config
func makeVisitor(fn func(string, os.FileInfo, error, *Cluster, SiteConfig, log.Logger) error,
	c *Cluster, s SiteConfig, sl log.Logger) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s, sl)
	}
}

func Verify(c *Cluster, s SiteConfig, sl log.Logger) {
	sl.Log("level", "INFO", "msg", "starting verifier")

	rand.Seed(int64(time.Now().Unix()) + int64(int(s.Port)))
	var jitter int
	var base_time = s.VerifierSleep
	for {
		// avoid thundering herd
		jitter = rand.Intn(5)
		time.Sleep(time.Duration(base_time+jitter) * time.Second)
		sl.Log("level", "INFO", "msg", "verifier starting at the top")

		root := s.UploadDirectory
		err := randwalk.Walk(root, makeVisitor(visit, c, s, sl))
		if err != nil {
			sl.Log("level", "WARN", "msg", "randwalk.Walk() returned error",
				"error", err.Error())
		}
		verifierPass.Add(1)
		// offset should only be applied on the first pass through
	}
}
