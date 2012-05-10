package main

import (
  "net/http"
	"./views"
)

func main() {
  http.HandleFunc("/", views.IndexHandler)
	http.HandleFunc("/add/",views.AddHandler)
  http.ListenAndServe(":8080", nil)
}
