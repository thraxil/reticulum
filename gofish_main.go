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

type ConfigData struct {
	Port int64
	UUID string
	Name string
}

func main() {
	var config string
	flag.StringVar(&config, "config", "./config.json", "JSON config file")
  flag.Parse()

	file, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatal(err)
	}

	f := ConfigData{}
	json.Unmarshal(file, &f)

	http.HandleFunc("/", views.AddHandler)
	http.HandleFunc("/image/", views.ServeImageHandler)
	http.ListenAndServe(fmt.Sprintf(":%d", f.Port), nil)
}
