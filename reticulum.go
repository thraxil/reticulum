package main

import (
	"./models"
	"./resize_worker"
	"./views"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"time"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, *models.Cluster, models.SiteConfig, models.SharedChannels),
	cluster *models.Cluster, siteconfig models.SiteConfig,
	channels models.SharedChannels) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, cluster, siteconfig, channels)
	}
}

func Log(handler http.Handler, logger *syslog.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		handler.ServeHTTP(w, r)
		t1 := time.Now()
		logger.Info(fmt.Sprintf("%s %s %s [%v]", r.RemoteAddr, r.Method, r.URL, t1.Sub(t0)))
	})
}

func main() {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	sl.Info("starting up")

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
		ResizeQueue: make(chan resize_worker.ResizeRequest),
	}

	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go resize_worker.ResizeWorker(channels.ResizeQueue)
	}

	// set up HTTP Handlers
	http.HandleFunc("/", makeHandler(views.AddHandler, cluster, siteconfig, channels))
	http.HandleFunc("/stash/", makeHandler(views.StashHandler, cluster, siteconfig, channels))
	http.HandleFunc("/image/", makeHandler(views.ServeImageHandler, cluster, siteconfig, channels))
	http.HandleFunc("/retrieve/", makeHandler(views.RetrieveHandler, cluster, siteconfig, channels))
	http.HandleFunc("/announce/", makeHandler(views.AnnounceHandler, cluster, siteconfig, channels))
	http.HandleFunc("/favicon.ico", views.FaviconHandler)

	// everything is ready, let's go
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), Log(http.DefaultServeMux, sl))
}
