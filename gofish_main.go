package main

import (
	"./views"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func makeHandler(fn func(http.ResponseWriter, *http.Request, views.ConfigData), cfg views.ConfigData) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, cfg)
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

	f := views.ConfigData{}
	json.Unmarshal(file, &f)

	http.HandleFunc("/", makeHandler(views.AddHandler, f))
	http.HandleFunc("/image/", makeHandler(views.ServeImageHandler, f))
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), nil)
}
