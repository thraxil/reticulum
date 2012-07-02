package views

import (
	"../models"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/thraxil/resize"
	//  "../../resize"
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

func ServeImageHandler(w http.ResponseWriter, r *http.Request, world *models.World) {
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

	baseDir := "uploads/" + hashStringToPath(ahash)
	path := baseDir + "/full.jpg"
	sizedPath := baseDir + "/" + size + ".jpg"

	contents, err := ioutil.ReadFile(sizedPath)
	if err == nil {
		// we've got it, so serve it directly
		w.Header().Set("Content-Type", "image/jpg")
		w.Write(contents)
		return
	}

	// we don't have a scaled version, so try to get the full version
	// resize it, write a cached version, then serve it
	origFile, err := os.Open(path)
	defer origFile.Close()
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}

	m, err := jpeg.Decode(origFile)
	if err != nil {
		http.Error(w, "error decoding image", 500)
	}

	outputImage := resize.Resize(m, size)
	wFile, err := os.OpenFile(sizedPath, os.O_CREATE|os.O_RDWR, 0644)
	defer wFile.Close()
	if err != nil {
		// what do we do if we can't write?
		// we still have the resized image, so we can serve the response
		// we just can't cache it. 
	}
	jpeg.Encode(wFile, outputImage, nil)

	w.Header().Set("Content-Type", "image/jpg")
	jpeg.Encode(w, outputImage, nil)
}

func AddHandler(w http.ResponseWriter, r *http.Request, world *models.World) {
	if r.Method == "POST" {
		i, _, _ := r.FormFile("image")
		h := sha1.New()
		d, _ := ioutil.ReadAll(i)
		io.WriteString(h, string(d))
		path := "uploads/" + hashToPath(h.Sum(nil))
		os.MkdirAll(path, 0755)
		fullpath := path + "full.jpg"
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

type AnnounceResponse struct {
	Nickname string
	UUID string
	Location string
	Writeable bool
	BaseUrl string
	Neighbors []models.NodeData
}

func AnnounceHandler(w http.ResponseWriter, r *http.Request, world *models.World) {
	ar := AnnounceResponse{
	  Nickname: world.Myself.Nickname,
		UUID: world.Myself.UUID,
    Location: world.Myself.Location,
  	Writeable: world.Myself.Writeable,
  	BaseUrl: world.Myself.BaseUrl,
	Neighbors: world.Neighbors,
	}
	b, err := json.Marshal(ar)
	if err != nil {
		fmt.Println("error:", err)
	}
	w.Write(b)
}
