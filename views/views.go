package views

import (
	"crypto/sha1"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
)

type Page struct {
	Title string
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	t, _ := template.ParseFiles("templates/" + tmpl + ".html")
	t.Execute(w, p)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "index"}
	renderTemplate(w, "index", &p)
}

func AddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		p := Page{Title: "uploaded image"}
		i, ih, _ := r.FormFile("image")
		h := sha1.New()
		d, _ := ioutil.ReadAll(i)
		io.WriteString(h, d.String())
		fmt.Printf("% x", h.Sum(nil))
		fmt.Println(ih.Filename)
		renderTemplate(w, "add", &p)
	} else {
		p := Page{Title: "upload image"}
		renderTemplate(w, "add", &p)
	}
}
