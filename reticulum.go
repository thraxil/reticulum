package main

import (
	"encoding/json"
	_ "expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"time"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, Context), ctx Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, ctx)
	}
}

func Log(handler http.Handler, node_name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				log.Println("Server Error:", rc)
				log.Println(r.URL.String())
			}
		}()
		t0 := time.Now()
		handler.ServeHTTP(w, r)
		t1 := time.Now()
		log.Println(fmt.Sprintf("%s: %s %s %s [%v]", node_name, r.RemoteAddr, r.Method,
			r.URL, t1.Sub(t0)))
	})
}

var VERIFY_OFFSET = 0
var VERIFY_SKIP = 0

func main() {
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

	siteconfig := f.MyConfig()

	gcp := &GroupCacheProxy{}
	c := NewCluster(f.MyNode(), gcp, siteconfig.GroupcacheSize)
	for i := range f.Neighbors {
		c.AddNeighbor(f.Neighbors[i])
	}

	runtime.GOMAXPROCS(siteconfig.GoMaxProcs)

	// start our resize worker goroutines
	var channels = SharedChannels{
		ResizeQueue: make(chan ResizeRequest),
	}
	sl := STDLogger{}
	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go ResizeWorker(channels.ResizeQueue, sl, &siteconfig)
	}

	// start our gossiper
	go c.Gossip(int(f.Port), siteconfig.GossiperSleep, sl)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	VERIFY_OFFSET = r.Intn(10000)
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
	http.HandleFunc("/config/", makeHandler(ConfigHandler, ctx))
	http.HandleFunc("/join/", makeHandler(JoinHandler, ctx))
	http.HandleFunc("/favicon.ico", FaviconHandler)

	// everything is ready, let's go
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", f.Port), Log(http.DefaultServeMux, c.Myself.Nickname)))
}
