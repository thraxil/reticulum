package main

import (
	"./views"
	"net/http"
)

func main() {
	http.HandleFunc("/", views.AddHandler)
	http.HandleFunc("/image/", views.ServeImageHandler)
	http.ListenAndServe(":8080", nil)
}
