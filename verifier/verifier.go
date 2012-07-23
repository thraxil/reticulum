package verifier

import (
	"../cluster"
	"../models"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func basename(path string) string {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	return filename[:len(filename)-len(ext)]
}

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

func visit(path string, f os.FileInfo, err error, c *cluster.Cluster,
	s models.SiteConfig) error {
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

	//    VERIFY PHASE
	hash, err := hashFromPath(path)
	if err != nil {
		return nil
	}

	h := sha1.New()
	d, _ := ioutil.ReadFile(path)
	io.WriteString(h, string(d))
	ahash := fmt.Sprintf("%x", h.Sum(nil))
	nodes_to_check := c.ReadOrder(hash)

	if hash != ahash {
		fmt.Printf("image %s appears to be corrupted!\n", path)
		// trust that the hash was correct on upload
		// ask other nodes for a copy
		var repaired = false
		for _, n := range nodes_to_check {
			if n.UUID == c.Myself.UUID {
				// skip ourself, since we know we are corrupt
				continue
			}
			img, err := n.RetrieveImage(hash, "full", extension)
			if err != nil {
				// doesn't have it
				fmt.Printf("node %s does not have a copy of the desired image\n", n.Nickname)
				continue
			} else {
				// double check that what we get is indeed right
				hn := sha1.New()
				io.WriteString(hn, string(img))
				nhash := fmt.Sprintf("%x", hn.Sum(nil))
				if nhash != hash {
					// this is not the correct one either!
					continue
				}
				// replace the full-size with a corrected one
				f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
				if err != nil {
					// can't open for writing!
					fmt.Printf("could not open for writing: %s, %s\n",path,err)
					return err
				}
				defer f.Close()
				_, err = f.Write(img)
				if err != nil {
					fmt.Printf("could not write: %s, %s\n",path,err)
					return err
				}
				repaired = true
				break
			}
		}
		if repaired {
			// cached sizes may have been created off the broken one
			// and the easiest solution is to take off
			// and nuke the site from orbit. It's the only way to be sure.
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
		} else {
			fmt.Printf("could not repair corrupted image: %s\n", path)
			// return here so we don't try to rebalance a corrupted image
			return errors.New("unrepairable image")
		}
	}

	//    REBALANCE PHASE
	var delete_local = true
	var satisfied = false
	var found_replicas = 0
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
					fmt.Printf("replicated %s\n", path)
					found_replicas++
				} else {
					// couldn't stash to that node. not writeable perhaps.
					// not really our problem to deal with, but we do want
					// to make sure that another node gets a copy
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
		fmt.Printf("could not replicate %s to %d nodes",path,s.Replication)
	} else {
		fmt.Printf("%s has full replica set\n", path)
	}

	if delete_local {
		// our node is not at the front of the list, so 
		// we have an excess copy. clean that up and make room!
		err := os.RemoveAll(filepath.Dir(path))
		if err != nil {
			fmt.Printf("could not clear out excess replica: %s\n", path)
			fmt.Println(err.Error())
		} else {
			fmt.Printf("cleared excess replica: %s\n", path)
		}
	}

	// slow things down a little to keep server load down
	var base_time = 1
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time+jitter) * time.Second)

	return nil
}

// makes a closure that has access to the cluster and config
func makeVisitor(fn func(string, os.FileInfo, error, *cluster.Cluster, models.SiteConfig) error,
	c *cluster.Cluster, s models.SiteConfig) func(path string, f os.FileInfo, err error) error {
	return func(path string, f os.FileInfo, err error) error {
		return fn(path, f, err, c, s)
	}
}

func Verify(c *cluster.Cluster, s models.SiteConfig) {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	sl.Info("starting verifier")

	rand.Seed(int64(time.Now().Unix()) + int64(int(s.Port)))
	var jitter int
	var base_time = 1
	for {
		// avoid thundering herd
		jitter = rand.Intn(5)
		time.Sleep(time.Duration(base_time+jitter) * time.Second)
		sl.Info("verifier starting at the top")

		root := s.UploadDirectory
		err := filepath.Walk(root, makeVisitor(visit, c, s))
		if err != nil {
			fmt.Printf("filepath.Walk() returned %v\n", err)
		}
	}
}
