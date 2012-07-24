package verifier

import (
	"../cluster"
	"../models"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// part of the path that's not a directory or extension
func basename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

// convert some/base/12/34/45/56/67/.../file.jpg to 1234455667
func hashFromPath(path string) (string, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir, "/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return "", errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts)-20:], "")
	if len(hash) != 40 {
		return "", errors.New("invalid hash")
	}
	return hash, nil
}

// checks the image for corruption
// if it is corrupt, try to repair
func verify(path string, extension string, hash string, ahash string,
	c *cluster.Cluster, sl *syslog.Writer) error {
	//    VERIFY PHASE
	if hash != ahash {
		sl.Warning(fmt.Sprintf("image %s appears to be corrupted!\n", path))
		// trust that the hash was correct on upload
		// ask other nodes for a copy
		repaired, err := repair_image(path, extension, hash, c, sl)
		if err != nil {
			return err
		}
		if repaired {
			err := clear_cached(path, extension)
			if err != nil {
				return err
			}
		} else {
			sl.Err(fmt.Sprintf("could not repair corrupted image: %s\n", path))
			// return here so we don't try to rebalance a corrupted image
			return errors.New("unrepairable image")
		}
	}
	return nil
}

// do our best to repair the image
func repair_image(path string, extension string, hash string,
	c *cluster.Cluster, sl *syslog.Writer) (bool, error) {
	nodes_to_check := c.ReadOrder(hash)
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {
			// skip ourself, since we know we are corrupt
			continue
		}
		img, err := n.RetrieveImage(hash, "full", extension)
		if err != nil {
			// doesn't have it
			sl.Info(fmt.Sprintf("node %s does not have a copy of the desired image\n", n.Nickname))
			continue
		} else {
			if !doublecheck_replica(img, hash) {
				// the copy from that node isn't right either
				continue
			}
			// replace the full-size with a corrected one
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				// can't open for writing!
				sl.Err(fmt.Sprintf("could not open for writing: %s, %s\n",path,err))
				return false, err
			}
			defer f.Close()
			_, err = f.Write(img)
			if err != nil {
				sl.Err(fmt.Sprintf("could not write: %s, %s\n",path,err))
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func doublecheck_replica(img []byte, hash string) bool {
	hn := sha1.New()
	io.WriteString(hn, string(img))
	nhash := fmt.Sprintf("%x", hn.Sum(nil))
	return nhash == hash
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
		if file.IsDir() { continue }
		if file.Name() == "full" + extension { continue }
		err := os.Remove(filepath.Join(filepath.Dir(path), file.Name()))
		successful_purge = successful_purge && (err == nil)
	}
	if !successful_purge {
		// one or more cached sizes were not deleted
		return errors.New("could not clear potentially corrupted scaled image")
	}
	return nil
}

// check that the image is stored in at least Replication nodes
// and, if at all possible, those should be the ones at the front
// of the list
func rebalance(path string, extension string, hash string, c *cluster.Cluster,
	s models.SiteConfig, sl *syslog.Writer) error {
	//    REBALANCE PHASE
	var delete_local = true
	var satisfied = false
	var found_replicas = 0
	nodes_to_check := c.ReadOrder(hash)
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {
			// don't need to delete it
			delete_local = false
			found_replicas++
		} else {
			img_info, err := n.RetrieveImageInfo(hash, "full", extension[1:])

			if err == nil && img_info.Local {
				// node should have it. node has it. cool.
				found_replicas++
			} else {
				// that node should have a copy, but doesn't so stash it
				if n.Stash(path) {
					sl.Info(fmt.Sprintf("replicated %s\n", path))
					found_replicas++
				} else {
					// couldn't stash to that node. not writeable perhaps.
					// not really our problem to deal with, but we do want
					// to make sure that another node gets a copy
					// so we don't increment found_replicas
				}
			}
		}
		if found_replicas >= s.Replication {
			// nothing more to do. other nodes that have excess
			// copies are responsible for deletion. Our job
			// is just to make sure the first N nodes have a copy
			satisfied = true
			break
		}
	}
	if !satisfied {
		sl.Warning(fmt.Sprintf("could not replicate %s to %d nodes",path,s.Replication))
	} else {
		sl.Info(fmt.Sprintf("%s has full replica set\n", path))
	}
	if delete_local {
		clean_up_excess_replica(path, sl)
	}
	return nil
}

// our node is not at the front of the list, so 
// we have an excess copy. clean that up and make room!
func clean_up_excess_replica(path string, sl *syslog.Writer) {
	err := os.RemoveAll(filepath.Dir(path))
	if err != nil {
		sl.Err(fmt.Sprintf("could not clear out excess replica: %s\n", path))
		sl.Err(err.Error())
	} else {
		sl.Info(fmt.Sprintf("cleared excess replica: %s\n", path))
	}
}

func visit(path string, f os.FileInfo, err error, c *cluster.Cluster,
	s models.SiteConfig, sl *syslog.Writer) error {
	// all we care about is the "full" version of each
	if f.IsDir() {
		return nil
	}
	if basename(path) != "full" {
		return nil
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
	d, _ := ioutil.ReadFile(path)
	io.WriteString(h, string(d))
	ahash := fmt.Sprintf("%x", h.Sum(nil))

	err = verify(path, extension, hash, ahash, c, sl)
	if err != nil {
		return err
	}
	err = rebalance(path, extension, hash, c, s, sl)
	if err != nil {
		return err
	}

	// slow things down a little to keep server load down
	var base_time = s.VerifierSleep
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time+jitter) * time.Second)

	return nil
}

// makes a closure that has access to the cluster and config
func makeVisitor(fn func(string, os.FileInfo, error, *cluster.Cluster, models.SiteConfig, *syslog.Writer) error,
	c *cluster.Cluster, s models.SiteConfig, sl *syslog.Writer) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s, sl)
	}
}

func Verify(c *cluster.Cluster, s models.SiteConfig, sl *syslog.Writer) {
	sl.Info("starting verifier")

	rand.Seed(int64(time.Now().Unix()) + int64(int(s.Port)))
	var jitter int
	var base_time = s.VerifierSleep
	for {
		// avoid thundering herd
		jitter = rand.Intn(5)
		time.Sleep(time.Duration(base_time+jitter) * time.Second)
		sl.Info("verifier starting at the top")

		root := s.UploadDirectory
		err := filepath.Walk(root, makeVisitor(visit, c, s, sl))
		if err != nil {
			sl.Info(fmt.Sprintf("filepath.Walk() returned %v\n", err))
		}
	}
}
