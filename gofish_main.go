package main

import (
	"./views"
	"./models"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, models.World), world models.World) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, world)
	}
}

func main() {
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

	world := models.NewWorld(f.MyNode())

	http.HandleFunc("/", makeHandler(views.AddHandler, *world))
	http.HandleFunc("/image/", makeHandler(views.ServeImageHandler, *world))
	http.HandleFunc("/announce/", makeHandler(views.AnnounceHandler, *world))
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), nil)
}
