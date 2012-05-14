package main

import (
	"./views"
	"net/http"
)

func main() {
	http.HandleFunc("/", views.IndexHandler)
	http.HandleFunc("/add/", views.AddHandler)
	http.ListenAndServe(":8080", nil)
}
