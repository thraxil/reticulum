package views

import (
	"html/template"
	"net/http"
)

type Page struct {
    Title string
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/index.html")
	p := Page{Title: "index"}
	t.Execute(w,p)
}

func AddHandler(w http.ResponseWriter, r *http.Request) {
  t, _ := template.ParseFiles("templates/add.html")
	p := Page{Title: "upload image"}
	t.Execute(w,p)
}
