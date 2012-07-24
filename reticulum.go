package main

import (
	"./cluster"
	"./models"
	"./resize_worker"
	"./verifier"
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

func makeHandler(fn func(http.ResponseWriter, *http.Request, *cluster.Cluster,
	models.SiteConfig, models.SharedChannels, *syslog.Writer),
	c *cluster.Cluster, siteconfig models.SiteConfig,
	channels models.SharedChannels, sl *syslog.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, c, siteconfig, channels, sl)
	}
}

func Log(handler http.Handler, logger *syslog.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				fmt.Println("Server Error", rc)
				logger.Err(fmt.Sprintf("%s",r.URL))
			}
		}()
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

	c := cluster.NewCluster(f.MyNode())
	for i := range f.Neighbors {
		c.AddNeighbor(f.Neighbors[i])
	}

	siteconfig := f.MyConfig()

	// start our resize worker goroutines
	var channels = models.SharedChannels{
		ResizeQueue: make(chan resize_worker.ResizeRequest),
	}

	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go resize_worker.ResizeWorker(channels.ResizeQueue, sl)
	}

	// start our gossiper
	go c.Gossip(int(f.Port), siteconfig.GossiperSleep, sl)

	go verifier.Verify(c, siteconfig, sl)

	// set up HTTP Handlers
	http.HandleFunc("/", makeHandler(views.AddHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/stash/", makeHandler(views.StashHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/image/", makeHandler(views.ServeImageHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/retrieve/", makeHandler(views.RetrieveHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/retrieve_info/", makeHandler(views.RetrieveInfoHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/announce/", makeHandler(views.AnnounceHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/status/", makeHandler(views.StatusHandler, c, siteconfig, channels, sl))
	http.HandleFunc("/favicon.ico", views.FaviconHandler)

	// everything is ready, let's go
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), Log(http.DefaultServeMux, sl))
}
