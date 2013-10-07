package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"runtime"
	"time"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, Context), ctx Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, ctx)
	}
}

func Log(handler http.Handler, logger *syslog.Writer, node_name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				fmt.Println("Server Error", rc)
				logger.Err(fmt.Sprintf("%s", r.URL))
			}
		}()
		t0 := time.Now()
		handler.ServeHTTP(w, r)
		t1 := time.Now()
		logger.Info(fmt.Sprintf("%s: %s %s %s [%v]", node_name, r.RemoteAddr, r.Method,
			r.URL, t1.Sub(t0)))
	})
}

func main() {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	sl.Info("starting up")

	// read the config file
	var configfile string
	flag.StringVar(&configfile, "config", "./config.json", "JSON config file")
	flag.Parse()

	file, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Fatal(err)
	}

	f := ConfigData{}
	err = json.Unmarshal(file, &f)
	if err != nil {
		log.Fatal(err)
	}

	c := NewCluster(f.MyNode())
	for i := range f.Neighbors {
		c.AddNeighbor(f.Neighbors[i])
	}

	siteconfig := f.MyConfig()

	runtime.GOMAXPROCS(siteconfig.GoMaxProcs)

	// start our resize worker goroutines
	var channels = SharedChannels{
		ResizeQueue: make(chan ResizeRequest),
	}

	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go ResizeWorker(channels.ResizeQueue, sl, &siteconfig)
	}

	// start our gossiper
	go c.Gossip(int(f.Port), siteconfig.GossiperSleep, sl)

	go Verify(c, siteconfig, sl)

	ctx := Context{Cluster: c, Cfg: siteconfig, Ch: channels, SL: sl}
	// set up HTTP Handlers
	http.HandleFunc("/", makeHandler(AddHandler, ctx))
	http.HandleFunc("/stash/", makeHandler(StashHandler, ctx))
	http.HandleFunc("/image/", makeHandler(ServeImageHandler, ctx))
	http.HandleFunc("/retrieve/", makeHandler(RetrieveHandler, ctx))
	http.HandleFunc("/retrieve_info/", makeHandler(RetrieveInfoHandler, ctx))
	http.HandleFunc("/announce/", makeHandler(AnnounceHandler, ctx))
	http.HandleFunc("/status/", makeHandler(StatusHandler, ctx))
	http.HandleFunc("/favicon.ico", FaviconHandler)

	// everything is ready, let's go
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", f.Port), Log(http.DefaultServeMux, sl, c.Myself.Nickname)))
}
