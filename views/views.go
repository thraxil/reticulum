package views

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/thraxil/resize"
	"html/template"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Page struct {
	Title string
}

type ImageData struct {
	Hash   string
	Length int
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	t, _ := template.ParseFiles("templates/" + tmpl + ".html")
	t.Execute(w, p)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	p := Page{Title: "index"}
	renderTemplate(w, "index", &p)
}

func hashToPath(h []byte) string {
	buffer := bytes.NewBufferString("")
	for i := range h {
		fmt.Fprint(buffer, fmt.Sprintf("%02x/", h[i]))
	}
	return buffer.String()
}

func hashStringToPath(h string) string {
	var parts []string
	for i := range h {
		if (i % 2) != 0 {
			parts = append(parts, h[i-1:i+1])
		}
	}
	return strings.Join(parts, "/")
}

func ServeImageHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) < 5) || (parts[1] != "image") {
		http.Error(w, "bad request", 404)
		return
	}
	ahash := parts[2]
	size := parts[3]
	filename := parts[4]
	if filename == "" {
		filename = "image.jpg"
	}

	if len(ahash) != 40 {
		http.Error(w, "bad hash", 404)
		return
	}

	path := "uploads/" + hashStringToPath(ahash) + "/image.jpg"

	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	defer file.Close()

	m, err := jpeg.Decode(file)
	if err != nil {
		http.Error(w, "error decoding image", 500)
	}

	outputImage := resize.Resize(m, size)
	w.Header().Set("Content-Type", "image/jpg")
	jpeg.Encode(w, outputImage, nil)
	//	w.Write(contents)
}

func AddHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		i, _, _ := r.FormFile("image")
		h := sha1.New()
		d, _ := ioutil.ReadAll(i)
		io.WriteString(h, string(d))
		path := "uploads/" + hashToPath(h.Sum(nil))
		os.MkdirAll(path, 0755)
		fullpath := path + "image.jpg"
		f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		n, _ := f.Write(d)
		id := ImageData{
			Hash:   fmt.Sprintf("%x", h.Sum(nil)),
			Length: n,
		}
		b, err := json.Marshal(id)
		if err != nil {
			fmt.Println("error:", err)
		}
		w.Write(b)
	} else {
		p := Page{Title: "upload image"}
		renderTemplate(w, "add", &p)
	}
}
