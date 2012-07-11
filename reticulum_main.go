package main

import (
	"./models"
	"./views"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *models.Cluster, models.SiteConfig, models.SharedChannels), 
	cluster *models.Cluster, siteconfig models.SiteConfig,
	channels models.SharedChannels) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, cluster, siteconfig, channels)
	}
}

func main() {
	// read the config file
	var config string
	flag.StringVar(&config, "config", "./config.json", "JSON config file")
	flag.Parse()

	file, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatal(err)
	}

	f := models.ConfigData{}
	err = json.Unmarshal(file, &f)
	if err != nil {
		log.Fatal(err)
	}

	cluster := models.NewCluster(f.MyNode())
	for i := range f.Neighbors {
		cluster.AddNeighbor(f.Neighbors[i])
	}

	siteconfig := f.MyConfig()

	// start our resize worker goroutines
	var channels = models.SharedChannels{
		ResizeQueue:make(chan models.ResizeRequest),
	}

	for i := 0; i < 4; i++ {
		go views.ResizeWorker(channels.ResizeQueue)
	}
	
	// set up HTTP Handlers
	http.HandleFunc("/", makeHandler(views.AddHandler, cluster, siteconfig, channels))
	http.HandleFunc("/stash/", makeHandler(views.StashHandler, cluster, siteconfig, channels))
	http.HandleFunc("/image/", makeHandler(views.ServeImageHandler, cluster, siteconfig, channels))
	http.HandleFunc("/announce/", makeHandler(views.AnnounceHandler, cluster, siteconfig, channels))

	// everything is ready, let's go
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), nil)
}
