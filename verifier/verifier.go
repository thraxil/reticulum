package verifier

import (
	"../cluster"
	"../models"
	"path/filepath"
  "os"
  "fmt"
	"log"
	"log/syslog"
	"math/rand"
	"time"
)

func visit(path string, f os.FileInfo, err error) error {
	if f.IsDir() { return nil }
	fmt.Printf("Visited: %s\n", path)

	// TODO: sleep a bit in here
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
		// get a list of all the images on our node

		root := s.UploadDirectory
		err := filepath.Walk(root, visit)
		fmt.Printf("filepath.Walk() returned %v\n", err)

		// for each:
		//    VERIFY PHASE
		//    calculate hash
		//    compare to the path
		//    if different:
		//       corruption! 
		//       log it
		//       trust that the hash was correct on upload
		//       ask other nodes for a copy
		//       double check that what we get is indeed right
		//       replace the full-size with a corrected one
		//       delete all the cached sizes too since they
		//       may have been created off the broken one
    //    REBALANCE PHASE
		//    calculate the ring for the image
		//    check that each of the N nodes at the front of the ring have it
		//    for each:
		//       if it doesn't have it, stash it
		//    if we've verified that the first N nodes have it, and the current
		//      node is not in that first N, delete the local copy

	}
}
