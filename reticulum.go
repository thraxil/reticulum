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

func makeHandler(fn func(http.ResponseWriter, *http.Request, Context), ctx Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, ctx)
	}
}

func Log(handler http.Handler, node_name string, sl log.Logger) http.Handler {
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
			"node", node_name,
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
}

func main() {
	sl := NewSTDLogger()
	sl.Info("starting logger")

	// read the config file
	var configfile string
	flag.StringVar(&configfile, "config", "./config.json", "JSON config file")
	flag.Parse()

	file, err := ioutil.ReadFile(configfile)
	if err != nil {
		sl.Err(err.Error())
		os.Exit(1)
	}

	f := ConfigData{}
	err = json.Unmarshal(file, &f)
	if err != nil {
		sl.Err(err.Error())
		os.Exit(1)
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
	for i := 0; i < siteconfig.NumResizeWorkers; i++ {
		go ResizeWorker(channels.ResizeQueue, sl, &siteconfig)
	}

	// start our gossiper
	go c.Gossip(int(f.Port), siteconfig.GossiperSleep, sl)

	// seed the RNG
	rand.New(rand.NewSource(time.Now().UnixNano()))
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
	http.HandleFunc("/dashboard/", makeHandler(DashboardHandler, ctx))
	http.HandleFunc("/config/", makeHandler(ConfigHandler, ctx))
	http.HandleFunc("/join/", makeHandler(JoinHandler, ctx))
	http.HandleFunc("/favicon.ico", FaviconHandler)

	// everything is ready, let's go
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), Log(http.DefaultServeMux, c.Myself.Nickname, sl.writer))
}
