package main

import (
	"context"
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
func hashStringFromPath(path string) (string, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return "", errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return "", fmt.Errorf("invalid hash length: %d (%s)", len(hash), hash)
	}
	return hash, nil
}

// checks the image for corruption
// if it is corrupt, try to repair
func verifyImage(path string, extension string, hash *hash, ahash string,
	c *cluster, sl log.Logger) error {
	//    VERIFY PHASE
	if hash.String() != ahash {
		sl.Log("level", "WARN", "msg", "image appears to be corrupted!", "image", path)
		corruptedImages.Add(1)
		// trust that the hash was correct on upload
		// ask other nodes for a copy
		repaired, err := repairImage(path, extension, hash, c, sl)
		if err != nil {
			sl.Log("level", "ERR", "msg", "error attempting to repair image", "error", err.Error())
			return err
		}
		if repaired {
			repairedImages.Add(1)
			err := clearCached(path, extension)
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
func repairImage(path string, extension string, hash *hash,
	c *cluster, sl log.Logger) (bool, error) {
	nodesToCheck := c.ReadOrder(hash.String())
	for _, n := range nodesToCheck {
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

func checkImageOnNode(n nodeData, hash *hash, extension string, path string,
	c *cluster, sl log.Logger) (bool, bool, error) {
	s := resize.MakeSizeSpec("full")
	ri := &imageSpecifier{hash, s, extension}

	ctx := context.Background()
	img, err := n.RetrieveImage(ctx, ri)
	if err != nil {
		// doesn't have it
		sl.Log("level", "INFO", "node", n.Nickname,
			"msg", "node does not have a copy of the desired image")
		return true, true, nil
	}
	if !doublecheckReplica(img, hash) {
		// the copy from that node isn't right either
		return true, true, nil
	}
	return replaceImageWithCorrected(path, img, sl)

}

func doublecheckReplica(img []byte, hash *hash) bool {
	hn := sha1.New()
	io.WriteString(hn, string(img))
	nhash := fmt.Sprintf("%x", hn.Sum(nil))
	return nhash == hash.String()
}

// the only File methods that we care about
// makes it easier to mock
type fileIsh interface {
	IsDir() bool
	Name() string
}

// cached sizes may have been created off the broken one
// and the easiest solution is to take off
// and nuke the site from orbit. It's the only way to be sure.
func clearCached(path string, extension string) error {
	files, err := ioutil.ReadDir(filepath.Dir(path))
	if err != nil {
		// can't read the dir?!
		return err
	}
	var successfulPurge = true
	for _, file := range files {
		r := func(p string) error { return os.Remove(p) }
		err = clearCachedFile(file, path, extension, r)
		successfulPurge = successfulPurge && (err == nil)
	}
	if !successfulPurge {
		// one or more cached sizes were not deleted
		return errors.New("could not clear potentially corrupted scaled image")
	}
	return nil
}

type remover func(fullpath string) error

func clearCachedFile(file fileIsh, path, extension string, r remover) error {
	if file.IsDir() {
		return nil
	}
	if file.Name() == "full"+extension {
		return nil
	}
	return r(filepath.Join(filepath.Dir(path), file.Name()))
}

type imageRebalancer struct {
	c         *cluster
	s         siteConfig
	sl        log.Logger
	hash      *hash
	path      string
	extension string
}

func newImageRebalancer(path, extension string, hash *hash, c *cluster, s siteConfig, sl log.Logger) *imageRebalancer {
	return &imageRebalancer{c, s, sl, hash, path, extension}
}

// check that the image is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func (r imageRebalancer) Rebalance() error {
	//    REBALANCE PHASE
	if r.c == nil {
		r.sl.Log("level", "ERR", "msg", "rebalance was given a nil cluster")
		return errors.New("nil cluster")
	}
	nodesToCheck := r.c.ReadOrder(r.hash.String())
	satisfied, deleteLocal, foundReplicas := r.checkNodesForRebalance(nodesToCheck)
	if !satisfied {
		r.sl.Log("level", "WARN", "msg", "could not replicate",
			"image", r.path, "replication", r.s.Replication)
		rebalanceFailures.Add(1)
	} else {
		r.sl.Log("level", "INFO", "image", r.path,
			"msg", "full replica set",
			"foundReplicas", foundReplicas,
			"desired_replicas", r.s.Replication)
		rebalanceSuccesses.Add(1)
	}
	if satisfied && deleteLocal {
		cleanUpExcessReplica(r.path, r.sl)
		rebalanceCleanups.Add(1)
	}
	return nil
}

func (r imageRebalancer) checkNodesForRebalance(nodesToCheck []nodeData) (bool, bool, int) {
	var satisfied = false
	var foundReplicas = 0
	var deleteLocal = true
	// TODO: parallelize this
	for _, n := range nodesToCheck {
		if n.UUID == r.c.Myself.UUID {
			// don't need to delete it
			deleteLocal = false
			foundReplicas++
		} else {
			foundReplicas = foundReplicas + r.retrieveReplica(&n, satisfied)
		}
		if foundReplicas >= r.s.Replication {
			satisfied = true
		}
		if foundReplicas >= r.s.MaxReplication {
			// nothing more to do. other nodes that have excess
			// copies are responsible for deletion. Our job
			// is just to make sure the first N nodes have a copy
			return satisfied, deleteLocal, foundReplicas
		}
	}
	return satisfied, deleteLocal, foundReplicas
}

type stashableNode interface {
	Stash(imageSpecifier, string, backend) bool
	RetrieveImageInfo(context.Context, *imageSpecifier) (*imageInfoResponse, error)
}

func (r imageRebalancer) retrieveReplica(n stashableNode, satisfied bool) int {

	s := resize.MakeSizeSpec("full")
	ri := &imageSpecifier{r.hash, s, r.extension[1:]}

	ctx := context.Background()
	imgInfo, err := n.RetrieveImageInfo(ctx, ri)
	if err == nil && imgInfo != nil && imgInfo.Local {
		// node should have it. node has it. cool.
		return 1
	}
	// that node should have a copy, but doesn't so stash it
	if !satisfied {
		if n.Stash(*ri, "", r.s.Backend) {
			r.sl.Log("level", "INFO", "msg", "replicated", "image", r.path)
			return 1
		}
		// else: couldn't stash to that node. not writeable perhaps.
		// not really our problem to deal with, but we do want
		// to make sure that another node gets a copy
		// so we don't increment foundReplicas
	}

	return 0
}

// our node is not at the front of the list, so
// we have an excess copy. clean that up and make room!
func cleanUpExcessReplica(path string, sl log.Logger) {
	err := os.RemoveAll(filepath.Dir(path))
	if err != nil {
		sl.Log("level", "ERR", "msg", "could not clear out excess replica", "image", path,
			"error", err.Error())
	} else {
		sl.Log("level", "INFO", "msg", "cleared excess replica", "image", path)
	}
}

func visitPreChecks(path string, f fileIsh, err error, c *cluster, sl log.Logger) (bool, error) {
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

func visit(path string, f os.FileInfo, err error, c *cluster,
	s siteConfig, sl log.Logger) error {
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

	hash, err := hashFromPath(path)
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
	err = verifyImage(path, extension, hash, ahash, c, sl)
	if err != nil {
		return err
	}
	r := newImageRebalancer(path, extension, hash, c, s, sl)
	err = r.Rebalance()
	if err != nil {
		return err
	}
	c.verified(imageRecord{*hash, extension})
	// slow things down a little to keep server load down
	var baseTime = s.VerifierSleep
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(baseTime+jitter) * time.Second)
	return nil
}

// makes a closure that has access to the cluster and config
func makeVisitor(fn func(string, os.FileInfo, error, *cluster, siteConfig, log.Logger) error,
	c *cluster, s siteConfig, sl log.Logger) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s, sl)
	}
}

func verify(c *cluster, s siteConfig, sl log.Logger) {
	sl.Log("level", "INFO", "msg", "starting verifier")

	rand.Seed(int64(time.Now().Unix()) + int64(int(s.Port)))
	var jitter int
	var baseTime = s.VerifierSleep
	for {
		// avoid thundering herd
		jitter = rand.Intn(5)
		time.Sleep(time.Duration(baseTime+jitter) * time.Second)
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
