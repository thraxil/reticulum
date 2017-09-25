package main // import "github.com/thraxil/reticulum"

import (
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/go-kit/kit/log"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, sitecontext), ctx sitecontext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, ctx)
		totalRequests.Add(1)
	}
}

func logTop(handler http.Handler, nodeName string, sl log.Logger) http.Handler {
	sl = log.With(sl, "component", "web")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				sl.Log("level", "ERR", "path", r.URL.String(), "msg", rc)
			}
		}()
		t0 := time.Now()
		handler.ServeHTTP(w, r)
		t1 := time.Now()
		sl.Log(
			"level", "INFO",
			"node", nodeName,
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.String(),
			"time",
			fmt.Sprintf("%v", t1.Sub(t0)))
	})
}

var (
	resizeQueueLength  *expvar.Int
	numNeighbors       *expvar.Int
	neighborFailures   *expvar.Int
	corruptedImages    *expvar.Int
	repairedImages     *expvar.Int
	unrepairableImages *expvar.Int
	verifiedImages     *expvar.Int
	verifierPass       *expvar.Int

	rebalanceFailures  *expvar.Int
	rebalanceSuccesses *expvar.Int
	rebalanceCleanups  *expvar.Int

	servedLocally     *expvar.Int
	servedFromCluster *expvar.Int
	cacheHits         *expvar.Int
	cacheMisses       *expvar.Int
	resizeFailures    *expvar.Int
	servedByMagick    *expvar.Int
	servedScaled      *expvar.Int

	totalRequests *expvar.Int

	expUptime *expvar.Int
)

func init() {
	// prep expvar values
	resizeQueueLength = expvar.NewInt("resizeQueue")
	numNeighbors = expvar.NewInt("numNeighbors")
	neighborFailures = expvar.NewInt("neighborFailures")
	corruptedImages = expvar.NewInt("corruptedImages")
	repairedImages = expvar.NewInt("repairedImages")
	unrepairableImages = expvar.NewInt("unrepairableImages")
	verifiedImages = expvar.NewInt("verifiedImages")
	verifierPass = expvar.NewInt("verifierPass")
	rebalanceFailures = expvar.NewInt("rebalanceFailures")
	rebalanceSuccesses = expvar.NewInt("rebalanceSuccesses")
	rebalanceCleanups = expvar.NewInt("rebalanceCleanups")

	servedLocally = expvar.NewInt("servedLocally")
	servedFromCluster = expvar.NewInt("servedFromCluster")

	cacheHits = expvar.NewInt("cacheHits")
	cacheMisses = expvar.NewInt("cacheMisses")
	resizeFailures = expvar.NewInt("resizeFailures")
	servedByMagick = expvar.NewInt("servedByMagick")
	servedScaled = expvar.NewInt("servedScaled")

	totalRequests = expvar.NewInt("totalRequests")

	expUptime = expvar.NewInt("uptime")
}

func main() {
	sl := newSTDLogger()
	sl.Log("level", "INFO", "msg", "starting logger")

	// read the config file
	var configfile string
	flag.StringVar(&configfile, "config", "./config.json", "JSON config file")
	flag.Parse()

	file, err := ioutil.ReadFile(configfile)
	if err != nil {
		sl.Log("level", "ERR", "error", err.Error())
		os.Exit(1)
	}

	f := configData{}
	err = json.Unmarshal(file, &f)
	if err != nil {
		sl.Log("level", "ERR", "error", err.Error())
		os.Exit(1)
	}

	siteconfig := f.MyConfig()

	gcp := &groupCacheProxy{}
	c := newCluster(f.MyNode(), gcp, siteconfig.GroupcacheSize)
	for i := range f.Neighbors {
		c.AddNeighbor(f.Neighbors[i])
	}

	runtime.GOMAXPROCS(siteconfig.GoMaxProcs)

	go func() {
		// update uptime
		for {
			time.Sleep(1 * time.Second)
			expUptime.Add(1)
		}
	}()
	rwSL := log.With(sl, "component", "resize_worker")
	// start our resize worker goroutines
	var channels = sharedChannels{
		ResizeQueue: make(chan resizeRequest),
	}
	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go resizeWorker(channels.ResizeQueue, rwSL, &siteconfig)
	}

	gSL := log.With(sl, "component", "gossiper")
	// start our gossiper
	go c.Gossip(int(f.Port), siteconfig.GossiperSleep, gSL)

	// seed the RNG
	rand.New(rand.NewSource(time.Now().UnixNano()))
	vSL := log.With(sl, "component", "verifier")
	go verify(c, siteconfig, vSL)

	ctx := sitecontext{cluster: c, Cfg: siteconfig, Ch: channels, SL: sl}
	// set up HTTP Handlers
	http.HandleFunc("/", makeHandler(addHandler, ctx))
	http.HandleFunc("/stash/", makeHandler(stashHandler, ctx))
	http.HandleFunc("/image/", makeHandler(serveImageHandler, ctx))
	http.HandleFunc("/retrieve/", makeHandler(retrieveHandler, ctx))
	http.HandleFunc("/retrieve_info/", makeHandler(retrieveInfoHandler, ctx))
	http.HandleFunc("/announce/", makeHandler(announceHandler, ctx))
	http.HandleFunc("/status/", makeHandler(statusHandler, ctx))
	http.HandleFunc("/dashboard/", makeHandler(dashboardHandler, ctx))
	http.HandleFunc("/config/", makeHandler(configHandler, ctx))
	http.HandleFunc("/join/", makeHandler(joinHandler, ctx))
	http.HandleFunc("/favicon.ico", faviconHandler)

	// everything is ready, let's go
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), logTop(http.DefaultServeMux, c.Myself.Nickname, sl))
}
