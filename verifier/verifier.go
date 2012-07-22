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
	return filename[:len(filename) - len(ext)]
}

func hashFromPath(path string) (string, error) {
	dir := filepath.Dir(path)
	parts := strings.Split(dir,"/")
	// only want the last 20 parts
	if len(parts) < 20 {
		return "", errors.New("not enough parts")
	}
	hash := strings.Join(parts[len(parts) - 20:],"")
	if len(hash) != 40 {
		return "", errors.New("invalid hash")
	}
	return hash, nil
}

func visit(path string, f os.FileInfo, err error) error {
	// all we care about is the "full" version of each
	if f.IsDir() { return nil }
	if basename(path) != "full" { return nil }

	//    VERIFY PHASE
	hash, err := hashFromPath(path)
	if err != nil {
		return nil
	}
	fmt.Printf("Hash: %s\n", hash)

	h := sha1.New()
	d, _ := ioutil.ReadFile(path)
	io.WriteString(h, string(d))
	ahash := fmt.Sprintf("%x", h.Sum(nil))

	if hash != ahash {
		fmt.Printf("image %s appears to be corrupted!\n", path)
		//       trust that the hash was correct on upload
		//       ask other nodes for a copy
		//       double check that what we get is indeed right
		//       replace the full-size with a corrected one
		//       delete all the cached sizes too since they
		//       may have been created off the broken one
	}
  //    REBALANCE PHASE
	//    calculate the ring for the image
	//    check that each of the N nodes at the front of the ring have it
	//    for each:
	//       if it doesn't have it, stash it
	//    if we've verified that the first N nodes have it, and the current
	//      node is not in that first N, delete the local copy

	// TODO: sleep a bit in here
	var base_time = 1
	jitter := rand.Intn(5)
	time.Sleep(time.Duration(base_time + jitter) * time.Second)

	return nil
} 

func Verify (c *cluster.Cluster, s models.SiteConfig) {
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
		time.Sleep(time.Duration(base_time + jitter) * time.Second)
		sl.Info("verifier starting at the top")

		root := s.UploadDirectory
		err := filepath.Walk(root, visit)
		if err != nil {
			fmt.Printf("filepath.Walk() returned %v\n", err)
		}
	}
}
