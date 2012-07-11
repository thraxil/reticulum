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
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Page struct {
	Title      string
	RequireKey bool
}

type ImageData struct {
	Hash   string
	Length int
	Extension string
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

var decoders = map[string](func(io.Reader)(image.Image, error)){
	"jpg":jpeg.Decode,
	"gif":gif.Decode,
	"png":png.Decode,
}

var jpeg_options = jpeg.Options{Quality:90}

func ServeImageHandler(w http.ResponseWriter, r *http.Request, cluster *models.Cluster, siteconfig models.SiteConfig) {
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
	extension := filepath.Ext(filename)
	if len(ahash) != 40 {
		http.Error(w, "bad hash", 404)
		return
	}

	baseDir := siteconfig.UploadDirectory + hashStringToPath(ahash)
	dl, err := ioutil.ReadDir(baseDir)
	if err != nil {
		fmt.Println("error reading directory")
	}
	for _, dir := range dl {
		switch {
		case !dir.IsDir():
			fmt.Println(dir.Name())
		case dir.IsDir():
			fmt.Println("directory",dir.Name())
		}
	}
	path := baseDir + "/full" + extension
	sizedPath := baseDir + "/" + size + extension

	contents, err := ioutil.ReadFile(sizedPath)
	if err == nil {
		// we've got it, so serve it directly
		w.Header().Set("Content-Type", extmimes[extension[1:]])
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

	m, err := decoders[extension[1:]](origFile)
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
	w.Header().Set("Content-Type", extmimes[extension[1:]])
	if extension == ".jpg" {
		jpeg.Encode(wFile, outputImage, &jpeg_options)
		jpeg.Encode(w, outputImage, &jpeg_options)
		return
	} 
	if extension == ".gif" {
		// image/gif doesn't include an Encode()
		// so we'll use png for now. 
		// :(
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		return
	}
	if extension == ".png" {
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		return
	}
	
}

var mimeexts = map[string]string{
	"image/jpeg":"jpg",
	"image/gif":"gif",
	"image/png":"png",
}

var extmimes = map[string]string{
	"jpg":"image/jpeg",
	"gif":"image/gif",
	"png":"image/png",
}

func AddHandler(w http.ResponseWriter, r *http.Request, cluster *models.Cluster,
	siteconfig models.SiteConfig) {
	if r.Method == "POST" {
		if siteconfig.KeyRequired() {
			if !siteconfig.ValidKey(r.FormValue("key")) {
				http.Error(w, "invalid upload key", 403)
				return
			}
		}
		i, fh, _ := r.FormFile("image")
		h := sha1.New()
		d, _ := ioutil.ReadAll(i)
		io.WriteString(h, string(d))
		path := siteconfig.UploadDirectory + hashToPath(h.Sum(nil))
		os.MkdirAll(path, 0755)
		mimetype := fh.Header["Content-Type"][0]
		ext := mimeexts[mimetype]
		fullpath := path + "full." + ext
		f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		n, _ := f.Write(d)
		id := ImageData{
			Hash:   fmt.Sprintf("%x", h.Sum(nil)),
			Length: n,
			Extension: ext,
		}
		b, err := json.Marshal(id)
		if err != nil {
			fmt.Println("error:", err)
		}
		w.Write(b)
	} else {
		p := Page{
			Title:      "upload image",
			RequireKey: siteconfig.KeyRequired(),
		}
		renderTemplate(w, "add", &p)
	}
}

func StashHandler(w http.ResponseWriter, r *http.Request, cluster *models.Cluster,
	siteconfig models.SiteConfig) {
	if r.Method != "POST" {
		http.Error(w, "POST only", 400)
		return
	}
	i, fh, _ := r.FormFile("image")
	h := sha1.New()
	d, _ := ioutil.ReadAll(i)
	io.WriteString(h, string(d))
	path := siteconfig.UploadDirectory + hashToPath(h.Sum(nil))
	os.MkdirAll(path, 0755)
	mimetype := fh.Header["Content-Type"][0]
	ext := mimeexts[mimetype]
	fullpath := path + "full." + ext
	// TODO: if target file already exists, no need to overwrite
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	f.Write(d)
	fmt.Fprintln(w, "done")
}

type AnnounceResponse struct {
	Nickname  string
	UUID      string
	Location  string
	Writeable bool
	BaseUrl   string
	Neighbors []models.NodeData
}

func AnnounceHandler(w http.ResponseWriter, r *http.Request,
	cluster *models.Cluster, siteconfig models.SiteConfig) {
	if r.Method == "POST" {
		// another node is announcing themselves to us
		// if they are already in the Neighbors list, update as needed
		// TODO: this should use channels to make it concurrency safe, like Add
		if neighbor, ok := cluster.FindNeighborByUUID(r.FormValue("UUID")); ok {
			fmt.Println("found our neighbor")
			fmt.Println(neighbor.Nickname)
			if r.FormValue("Nickname") != "" {
				neighbor.Nickname = r.FormValue("Nickname")
			}
			if r.FormValue("Location") != "" {
				neighbor.Location = r.FormValue("Location")
			}
			if r.FormValue("BaseUrl") != "" {
				neighbor.BaseUrl = r.FormValue("BaseUrl")
			}
			if r.FormValue("Writeable") != "" {
				neighbor.Writeable = r.FormValue("Writeable") == "true"
			}
			neighbor.LastSeen = time.Now()
			// TODO: gossip enable by accepting the list of neighbors
			// from the client and merging that data in.
			// for now, just let it update its own entry

		} else {
			// otherwise, add them to the Neighbors list
			fmt.Println("adding neighbor")
			nd := models.NodeData{
				Nickname: r.FormValue("Nickname"),
				UUID:     r.FormValue("UUID"),
				BaseUrl:  r.FormValue("BaseUrl"),
				Location: r.FormValue("Location"),
			}
			if r.FormValue("Writeable") == "true" {
				nd.Writeable = true
			} else {
				nd.Writeable = false
			}
			nd.LastSeen = time.Now()
			cluster.AddNeighbor(nd)
		}
	}
	ar := AnnounceResponse{
		Nickname:  cluster.Myself.Nickname,
		UUID:      cluster.Myself.UUID,
		Location:  cluster.Myself.Location,
		Writeable: cluster.Myself.Writeable,
		BaseUrl:   cluster.Myself.BaseUrl,
		Neighbors: cluster.Neighbors,
	}
	b, err := json.Marshal(ar)
	if err != nil {
		fmt.Println("error:", err)
	}
	w.Write(b)
}
