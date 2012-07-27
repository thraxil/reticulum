package views

import (
	"../cluster"
	"../config"
	"../models"
	"../node"
	"../resize_worker"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"html/template"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log/syslog"
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
	Hash      string   `json:"hash"`
	Length    int      `json:"length"`
	Extension string   `json:"extension"`
	FullUrl   string   `json:"full_url"`
	Satisfied bool     `json:"satisfied"`
	Nodes     []string `json:"nodes"`
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page, s config.SiteConfig) {
	t, _ := template.ParseFiles(s.TemplateDirectory + "/" + tmpl + ".html")
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

var jpeg_options = jpeg.Options{Quality: 90}

func retrieveImage(c *cluster.Cluster, ahash string, size string, extension string) ([]byte, error) {
	// we don't have the full-size, so check the cluster
	nodes_to_check := c.ReadOrder(ahash)
	// this is where we go down the list and ask the other
	// nodes for the image
	// TODO: parallelize this
	for _, n := range nodes_to_check {
		if n.UUID == c.Myself.UUID {
			// checking ourself would be silly
			continue
		}
		img, err := n.RetrieveImage(ahash, size, extension)
		if err == nil {
			// got it, return it
			return img, nil
		}
		// that node didn't have it so we keep going
	}
	return nil, errors.New("not found in the cluster")
}

func ServeImageHandler(w http.ResponseWriter, r *http.Request, cls *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
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

	memcache_key := ahash + "/" + size + "/image" + extension
	// check memcached first
	item, err := mc.Get(memcache_key)
	if err == nil {
		sl.Info("Cache Hit")
		w.Header().Set("Content-Type", extmimes[extension[1:]])
		w.Write(item.Value)
		return
	}

	baseDir := siteconfig.UploadDirectory + hashStringToPath(ahash)
	path := baseDir + "/full" + extension
	sizedPath := baseDir + "/" + size + extension

	contents, err := ioutil.ReadFile(sizedPath)
	if err == nil {
		mc.Set(&memcache.Item{Key: memcache_key, Value: contents})
		// we've got it, so serve it directly
		w.Header().Set("Content-Type", extmimes[extension[1:]])
		w.Write(contents)
		return
	}

	_, err = ioutil.ReadFile(path)
	if err != nil {
		// we don't have the full-size on this node either
		// need to check the rest of the cluster
		img_data, err := retrieveImage(cls, ahash, size, extension[1:])
		if err != nil {
			// for now we just have to 404
			http.Error(w, "not found", 404)
		} else {
			mc.Set(&memcache.Item{Key: memcache_key, Value: img_data})
			w.Header().Set("Content-Type", extmimes[extension[1:]])
			w.Write(img_data)
		}
		return
	}

	// we do have the full-size, but not the scaled one
	// so resize it, cache it, and serve it.

	c := make(chan resize_worker.ResizeResponse)
	channels.ResizeQueue <- resize_worker.ResizeRequest{path, extension, size, c}
	result := <-c
	if !result.Success {
		http.Error(w, "could not resize image", 500)
		return
	}
	if result.Magick {
		// imagemagick did the resize, so we just spit out
		// the sized file
		w.Header().Set("Content-Type", extmimes[extension])
		img_contents, _ := ioutil.ReadFile(sizedPath)
		mc.Set(&memcache.Item{Key: memcache_key, Value: img_contents})
		w.Write(img_contents)
		return
	}
	outputImage := *result.OutputImage

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
		img_contents, _ := ioutil.ReadFile(sizedPath)
		mc.Set(&memcache.Item{Key: memcache_key, Value: img_contents})
		return
	}
	if extension == ".gif" {
		// image/gif doesn't include an Encode()
		// so we'll use png for now. 
		// :(
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		img_contents, _ := ioutil.ReadFile(sizedPath)
		mc.Set(&memcache.Item{Key: memcache_key, Value: img_contents})
		return
	}
	if extension == ".png" {
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		img_contents, _ := ioutil.ReadFile(sizedPath)
		mc.Set(&memcache.Item{Key: memcache_key, Value: img_contents})
		return
	}

}

var mimeexts = map[string]string{
	"image/jpeg": "jpg",
	"image/gif":  "gif",
	"image/png":  "png",
}

var extmimes = map[string]string{
	"jpg": "image/jpeg",
	"gif": "image/gif",
	"png": "image/png",
}

func AddHandler(w http.ResponseWriter, r *http.Request, c *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
	if r.Method == "POST" {
		if siteconfig.KeyRequired() {
			if !siteconfig.ValidKey(r.FormValue("key")) {
				http.Error(w, "invalid upload key", 403)
				return
			}
		}
		i, fh, _ := r.FormFile("image")
		defer i.Close()
		h := sha1.New()
		io.Copy(h, i)
		ahash := fmt.Sprintf("%x", h.Sum(nil))
		path := siteconfig.UploadDirectory + hashToPath(h.Sum(nil))
		os.MkdirAll(path, 0755)
		mimetype := fh.Header["Content-Type"][0]
		ext := mimeexts[mimetype]
		fullpath := path + "full." + ext
		f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		i.Seek(0, 0)
		io.Copy(f, i)
		// yes, the full-size for this image gets written to disk on
		// this node even if it may not be one of the "right" ones
		// for it to end up on. This isn't optimal, but is easy
		// and we can just let the verify/balance worker clean it up
		// at some point in the future.

		// now stash it to other nodes in the cluster too
		nodes := c.Stash(ahash, fullpath, siteconfig.Replication, siteconfig.MinReplication)

		id := ImageData{
			Hash:      ahash,
			Extension: ext,
			FullUrl:   "/image/" + ahash + "/full/image." + ext,
			Satisfied: len(nodes) >= siteconfig.MinReplication,
			Nodes:     nodes,
		}
		b, err := json.Marshal(id)
		if err != nil {
			sl.Err(err.Error())
		}
		w.Write(b)
	} else {
		p := Page{
			Title:      "upload image",
			RequireKey: siteconfig.KeyRequired(),
		}
		renderTemplate(w, "add", &p, siteconfig)
	}
}

type StatusPage struct {
	Title   string
	Config  config.SiteConfig
	Cluster cluster.Cluster
}

func StatusHandler(w http.ResponseWriter, r *http.Request, c *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
	p := StatusPage{
		Title:   "Status",
		Config:  siteconfig,
		Cluster: *c,
	}
	t, _ := template.ParseFiles(siteconfig.TemplateDirectory + "/status.html")
	t.Execute(w, p)
}

func StashHandler(w http.ResponseWriter, r *http.Request, c *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
	if r.Method != "POST" {
		http.Error(w, "POST only", 400)
		return
	}
	if !c.Myself.Writeable {
		http.Error(w, "non-writeable node", 400)
		return
	}

	i, fh, _ := r.FormFile("image")
	defer i.Close()
	h := sha1.New()
	io.Copy(h, i)

	path := siteconfig.UploadDirectory + hashToPath(h.Sum(nil))
	os.MkdirAll(path, 0755)
	ext := filepath.Ext(fh.Filename)
	fullpath := path + "full" + ext
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	i.Seek(0, 0)
	io.Copy(f, i)
}

func RetrieveInfoHandler(w http.ResponseWriter, r *http.Request, cls *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
	// request will look like /retrieve_info/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve_info") {
		http.Error(w, "bad request", 404)
		return
	}
	ahash := parts[2]
	extension := parts[4]
	var local = true
	if len(ahash) != 40 {
		http.Error(w, "bad hash", 404)
		return
	}

	baseDir := siteconfig.UploadDirectory + hashStringToPath(ahash)
	path := baseDir + "/full" + "." + extension
	_, err := os.Open(path)
	if err != nil {
		local = false
	}

	b, err := json.Marshal(node.ImageInfoResponse{ahash, extension, local})
	if err != nil {
		sl.Err(err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func RetrieveHandler(w http.ResponseWriter, r *http.Request, cls *cluster.Cluster,
	siteconfig config.SiteConfig, channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {

	// request will look like /retrieve/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve") {
		http.Error(w, "bad request", 404)
		return
	}
	ahash := parts[2]
	size := parts[3]
	extension := parts[4]

	if len(ahash) != 40 {
		http.Error(w, "bad hash", 404)
		return
	}

	baseDir := siteconfig.UploadDirectory + hashStringToPath(ahash)
	path := baseDir + "/full" + "." + extension
	sizedPath := baseDir + "/" + size + "." + extension

	contents, err := ioutil.ReadFile(sizedPath)
	if err == nil {
		// we've got it, so serve it directly
		w.Header().Set("Content-Type", extmimes[extension])
		w.Write(contents)
		return
	}
	_, err = ioutil.ReadFile(path)
	if err != nil {
		// we don't have the full-size on this node either
		http.Error(w, "not found", 404)
		return
	}
	// we do have the full-size, but not the scaled one
	// so resize it, cache it, and serve it.

	c := make(chan resize_worker.ResizeResponse)
	channels.ResizeQueue <- resize_worker.ResizeRequest{path, "." + extension, size, c}
	result := <-c
	if !result.Success {
		http.Error(w, "could not resize image", 500)
		return
	}
	if result.Magick {
		// imagemagick did the resize, so we just spit out
		// the sized file
		w.Header().Set("Content-Type", extmimes[extension])
		img_contents, _ := ioutil.ReadFile(sizedPath)
		w.Write(img_contents)
		return
	}
	outputImage := *result.OutputImage

	wFile, err := os.OpenFile(sizedPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		// what do we do if we can't write?
		// we still have the resized image, so we can serve the response
		// we just can't cache it. 
	}
	defer wFile.Close()
	w.Header().Set("Content-Type", extmimes[extension])
	if extension == "jpg" {
		jpeg.Encode(wFile, outputImage, &jpeg_options)
		jpeg.Encode(w, outputImage, &jpeg_options)
		return
	}
	if extension == "gif" {
		// image/gif doesn't include an Encode()
		// so we'll use png for now. 
		// :(
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		return
	}
	if extension == "png" {
		png.Encode(wFile, outputImage)
		png.Encode(w, outputImage)
		return
	}
}

func AnnounceHandler(w http.ResponseWriter, r *http.Request,
	c *cluster.Cluster, siteconfig config.SiteConfig,
	channels models.SharedChannels, sl *syslog.Writer, mc *memcache.Client) {
	if r.Method == "POST" {
		// another node is announcing themselves to us
		// if they are already in the Neighbors list, update as needed
		// TODO: this should use channels to make it concurrency safe, like Add
		if neighbor, ok := c.FindNeighborByUUID(r.FormValue("uuid")); ok {
			if r.FormValue("nickname") != "" {
				neighbor.Nickname = r.FormValue("nickname")
			}
			if r.FormValue("location") != "" {
				neighbor.Location = r.FormValue("location")
			}
			if r.FormValue("base_url") != "" {
				neighbor.BaseUrl = r.FormValue("base_url")
			}
			if r.FormValue("writeable") != "" {
				neighbor.Writeable = r.FormValue("writeable") == "true"
			}
			neighbor.LastSeen = time.Now()
			c.UpdateNeighbor(*neighbor)
			sl.Info("updated existing neighbor")
			// TODO: gossip enable by accepting the list of neighbors
			// from the client and merging that data in.
			// for now, just let it update its own entry

		} else {
			// otherwise, add them to the Neighbors list
			sl.Info("adding neighbor")
			nd := node.NodeData{
				Nickname: r.FormValue("nickname"),
				UUID:     r.FormValue("uuid"),
				BaseUrl:  r.FormValue("base_url"),
				Location: r.FormValue("location"),
			}
			if r.FormValue("writeable") == "true" {
				nd.Writeable = true
			} else {
				nd.Writeable = false
			}
			nd.LastSeen = time.Now()
			c.AddNeighbor(nd)
		}
	}
	ar := node.AnnounceResponse{
		Nickname:  c.Myself.Nickname,
		UUID:      c.Myself.UUID,
		Location:  c.Myself.Location,
		Writeable: c.Myself.Writeable,
		BaseUrl:   c.Myself.BaseUrl,
		Neighbors: c.Neighbors,
	}
	b, err := json.Marshal(ar)
	if err != nil {
		sl.Err(err.Error())
	}
	w.Write(b)
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	// just give it nothing to make it go away
	w.Write(nil)
}
