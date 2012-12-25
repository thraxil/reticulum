package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/thraxil/resize"
	"html/template"
	"image"
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

type Context struct {
	Cluster *Cluster
	Cfg     SiteConfig
	Ch      SharedChannels
	SL      *syslog.Writer
	MC      *memcache.Client
}

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

var jpeg_options = jpeg.Options{Quality: 90}

func setCacheHeaders(w http.ResponseWriter, extension string) http.ResponseWriter {
	w.Header().Set("Content-Type", extmimes[extension[1:]])
	w.Header().Set("Expires", time.Now().Add(time.Hour*24*365).Format(time.RFC1123))
	return w
}

func parsePathServeImage(w http.ResponseWriter, r *http.Request,
	ctx Context) (*ImageSpecifier, bool) {
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) < 5) || (parts[1] != "image") {
		http.Error(w, "bad request", 404)
		return nil, true
	}
	ahash, err := HashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "invalid hash", 404)
		return nil, true
	}
	size := parts[3]
	if size == "" {
		http.Error(w, "missing size", 404)
		return nil, true
	}
	s := resize.MakeSizeSpec(size)
	if s.String() != size {
		// force normalization of size spec
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+parts[4], 301)
		return nil, true
	}
	filename := parts[4]
	if filename == "" {
		filename = "image.jpg"
	}
	extension := filepath.Ext(filename)

	if extension == ".jpeg" {
		fixed_filename := strings.Replace(parts[4], ".jpeg", ".jpg", 1)
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+fixed_filename, 301)
		return nil, true
	}
	ri := &ImageSpecifier{ahash, s.String(), extension}
	return ri, false
}

func (ctx Context) serveFromCluster(ri *ImageSpecifier, w http.ResponseWriter) {
		// we don't have the full-size on this node either
		// need to check the rest of the cluster
		img_data, err := ctx.Cluster.RetrieveImage(ri.Hash, ri.Size, ri.Extension[1:])
		if err != nil {
			// for now we just have to 404
			http.Error(w, "not found", 404)
		} else {
			ctx.addToMemcache(ri.MemcacheKey(), img_data)
			w = setCacheHeaders(w, ri.Extension)
			w.Write(img_data)
		}
}

func (ctx Context) serveFromMemcache(ri *ImageSpecifier, w http.ResponseWriter) bool {
	// check memcached first
	item, err := ctx.MC.Get(ri.MemcacheKey())
	if err == nil {
		ctx.SL.Info("Cache Hit")
		w = setCacheHeaders(w, ri.Extension)
		w.Write(item.Value)
		return true
	}
	return false
}

func (ctx Context) serveDirect(ri *ImageSpecifier, w http.ResponseWriter) bool {
	contents, err := ioutil.ReadFile(ri.sizedPath(ctx.Cfg.UploadDirectory))
	if err == nil {
		ctx.addToMemcache(ri.MemcacheKey(), contents)
		// we've got it, so serve it directly
		w = setCacheHeaders(w, ri.Extension)
		w.Write(contents)
		return true
	}
	return false
}

func ServeImageHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	ri, handled := parsePathServeImage(w, r, ctx)
	if handled {
		return
	}

	if ctx.serveFromMemcache(ri, w) {
		return
	}

	if ctx.serveDirect(ri, w) {
		return
	}

	if !ctx.haveImageFullsizeLocally(ri) {
		ctx.serveFromCluster(ri, w)
		return
	}

	// we do have the full-size, but not the scaled one
	// so resize it, cache it, and serve it.
	if !ctx.locallyWriteable() {
		// but first, make sure we are writeable. If not,
		// we need to let another node in the cluster handle it.
		ctx.serveScaledFromCluster(ri, w)
		return
	}
	result := ctx.makeResizeJob(ri)
	if !result.Success {
		http.Error(w, "could not resize image", 500)
		return
	}
	if result.Magick {
		// imagemagick did the resize, so we just spit out
		// the sized file
		ctx.serveMagick(ri, w)
		return
	}
	ctx.serveScaledByExtension(ri, w, *result.OutputImage)
}

func (ctx Context) locallyWriteable() bool {
	return ctx.Cluster.Myself.Writeable
}

func (ctx Context) haveImageFullsizeLocally(ri *ImageSpecifier) bool {
	_, err := ioutil.ReadFile(ri.fullSizePath(ctx.Cfg.UploadDirectory))
	return err == nil
}

func (ctx Context) serveScaledFromCluster(ri *ImageSpecifier, w http.ResponseWriter) {
		img_data, err := ctx.Cluster.RetrieveImage(ri.Hash, ri.Size, ri.Extension[1:])
		if err != nil {
			// for now we just have to 404
			http.Error(w, "not found", 404)
		} else {
			ctx.addToMemcache(ri.MemcacheKey(), img_data)
			w = setCacheHeaders(w, ri.Extension)
			w.Write(img_data)
		}
		return
}

func (ctx Context) makeResizeJob(ri *ImageSpecifier) ResizeResponse {
	c := make(chan ResizeResponse)
	ctx.Ch.ResizeQueue <- ResizeRequest{ri.fullSizePath(ctx.Cfg.UploadDirectory), ri.Extension, ri.Size, c}
	result := <-c
	return result
}

func (ctx Context) serveMagick(ri *ImageSpecifier, w http.ResponseWriter) {
		img_contents, _ := ioutil.ReadFile(ri.sizedPath(ctx.Cfg.UploadDirectory))
		ctx.addToMemcache(ri.MemcacheKey(), img_contents)
		w = setCacheHeaders(w, ri.Extension)
		w.Write(img_contents)
}

func (ctx Context) serveScaledByExtension(ri *ImageSpecifier, w http.ResponseWriter,
	outputImage image.Image) {

	wFile, err := os.OpenFile(ri.sizedPath(ctx.Cfg.UploadDirectory), os.O_CREATE|os.O_RDWR, 0644)
	defer wFile.Close()
	if err != nil {
		// what do we do if we can't write?
		// we still have the resized image, so we can serve the response
		// we just can't cache it. 
	}
	w = setCacheHeaders(w, ri.Extension)

	if ri.Extension == ".jpg" {
		serveJpg(wFile, outputImage, w, ri.sizedPath(ctx.Cfg.UploadDirectory), ctx, ri.MemcacheKey())
		return
	}
	if ri.Extension == ".gif" {
		serveGif(wFile, outputImage, w, ri.sizedPath(ctx.Cfg.UploadDirectory), ctx, ri.MemcacheKey())
		return
	}
	if ri.Extension == ".png" {
		servePng(wFile, outputImage, w, ri.sizedPath(ctx.Cfg.UploadDirectory), ctx, ri.MemcacheKey())
		return
	}
}

func (ctx Context) addToMemcache(memcache_key string, img_contents []byte) {
	ctx.MC.Set(&memcache.Item{Key: memcache_key, Value: img_contents})
}

func serveJpg(wFile *os.File, outputImage image.Image, w http.ResponseWriter, sizedPath string, ctx Context,
	memcache_key string) {
	jpeg.Encode(wFile, outputImage, &jpeg_options)
	jpeg.Encode(w, outputImage, &jpeg_options)
	img_contents, _ := ioutil.ReadFile(sizedPath)
	ctx.addToMemcache(memcache_key, img_contents)
}

func serveGif(wFile *os.File, outputImage image.Image, w http.ResponseWriter, sizedPath string, ctx Context,
	memcache_key string) {
	// image/gif doesn't include an Encode()
	// so we'll use png for now. 
	// :(
	png.Encode(wFile, outputImage)
	png.Encode(w, outputImage)
	img_contents, _ := ioutil.ReadFile(sizedPath)
	ctx.addToMemcache(memcache_key, img_contents)
}

func servePng(wFile *os.File, outputImage image.Image, w http.ResponseWriter, sizedPath string, ctx Context,
	memcache_key string) {
	png.Encode(wFile, outputImage)
	png.Encode(w, outputImage)
	img_contents, _ := ioutil.ReadFile(sizedPath)
	ctx.addToMemcache(memcache_key, img_contents)
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

func AddHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	if r.Method == "POST" {
		if ctx.Cfg.KeyRequired() {
			if !ctx.Cfg.ValidKey(r.FormValue("key")) {
				http.Error(w, "invalid upload key", 403)
				return
			}
		}
		i, fh, _ := r.FormFile("image")
		defer i.Close()
		h := sha1.New()
		io.Copy(h, i)
		ahash, err := HashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
		if err != nil {
			http.Error(w, "bad hash", 500)
			return
		}
		path := ctx.Cfg.UploadDirectory + ahash.AsPath()
		os.MkdirAll(path, 0755)
		mimetype := fh.Header["Content-Type"][0]
		ext := mimeexts[mimetype]
		fullpath := path + "full." + ext
		f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		i.Seek(0, 0)
		io.Copy(f, i)
		size_hints := r.FormValue("size_hints")
		// yes, the full-size for this image gets written to disk on
		// this node even if it may not be one of the "right" ones
		// for it to end up on. This isn't optimal, but is easy
		// and we can just let the verify/balance worker clean it up
		// at some point in the future.

		// now stash it to other nodes in the cluster too
		nodes := ctx.Cluster.Stash(ahash, fullpath, size_hints, ctx.Cfg.Replication, ctx.Cfg.MinReplication)

		id := ImageData{
			Hash:      ahash.String(),
			Extension: ext,
			FullUrl:   "/image/" + ahash.String() + "/full/image." + ext,
			Satisfied: len(nodes) >= ctx.Cfg.MinReplication,
			Nodes:     nodes,
		}
		b, err := json.Marshal(id)
		if err != nil {
			ctx.SL.Err(err.Error())
		}
		w.Write(b)
	} else {
		p := Page{
			Title:      "upload image",
			RequireKey: ctx.Cfg.KeyRequired(),
		}
		t, _ := template.New("add").Parse(add_template)
		t.Execute(w, &p)
	}
}

type StatusPage struct {
	Title     string
	Config    SiteConfig
	Cluster   *Cluster
	Neighbors []NodeData
}

func StatusHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	p := StatusPage{
		Title:     "Status",
		Config:    ctx.Cfg,
		Cluster:   ctx.Cluster,
		Neighbors: ctx.Cluster.GetNeighbors(),
	}
	t, _ := template.New("status").Parse(status_template)
	t.Execute(w, p)
}

func StashHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	n := ctx.Cluster.Myself
	if r.Method != "POST" {
		http.Error(w, "POST only", 400)
		return
	}
	if !n.Writeable {
		http.Error(w, "non-writeable node", 400)
		return
	}

	i, fh, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "no image uploaded", 400)
		return
	}
	defer i.Close()
	h := sha1.New()
	io.Copy(h, i)
	ahash, err := HashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
	if err != nil {
		http.Error(w, "bad hash", 404)
		return
	}

	path := ctx.Cfg.UploadDirectory + ahash.AsPath()
	os.MkdirAll(path, 0755)
	ext := filepath.Ext(fh.Filename)
	fullpath := path + "full" + ext
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	i.Seek(0, 0)
	io.Copy(f, i)
	fmt.Fprint(w, "ok")
	// do any eager resizing in the background
	size_hints := r.FormValue("size_hints")
	go func() {
		sizes := strings.Split(size_hints, ",")
		for _, size := range sizes {
			c := make(chan ResizeResponse)
			ctx.Ch.ResizeQueue <- ResizeRequest{fullpath, ext, size, c}
			result := <-c
			if !result.Success {
				ctx.SL.Err("could not pre-resize")
			}
		}
	}()
}

func RetrieveInfoHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	// request will look like /retrieve_info/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve_info") {
		http.Error(w, "bad request", 404)
		return
	}
	ahash, err := HashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "bad hash", 404)
		return
	}
	extension := parts[4]
	var local = true
	baseDir := ctx.Cfg.UploadDirectory + ahash.AsPath()
	path := baseDir + "/full" + "." + extension
	_, err = os.Open(path)
	if err != nil {
		local = false
	}

	// if we aren't writeable, we can't resize locally
	// let them know this as early as possible
	size := parts[3]
	n := ctx.Cluster.Myself
	if size != "full" && !n.Writeable {
		// anything other than full-size, we can't do
		// if we don't have it already
		_, err = os.Open(baseDir + "/" + size + "." + extension)
		if err != nil {
			local = false
		}
	}

	b, err := json.Marshal(ImageInfoResponse{ahash.String(), extension, local})
	if err != nil {
		ctx.SL.Err(err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func RetrieveHandler(w http.ResponseWriter, r *http.Request, ctx Context) {

	// request will look like /retrieve/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve") {
		http.Error(w, "bad request", 404)
		return
	}
	ahash, err := HashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "bad hash", 404)
		return
	}
	size := parts[3]
	extension := parts[4]

	baseDir := ctx.Cfg.UploadDirectory + ahash.AsPath()
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

	// if we aren't writeable, we can't resize locally though.
	// 404 and let another node handle it
	n := ctx.Cluster.Myself
	if !n.Writeable {
		http.Error(w, "could not resize image", 404)
		return
	}

	c := make(chan ResizeResponse)
	ctx.Ch.ResizeQueue <- ResizeRequest{path, "." + extension, size, c}
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

func AnnounceHandler(w http.ResponseWriter, r *http.Request, ctx Context) {
	if r.Method == "POST" {
		// another node is announcing themselves to us
		// if they are already in the Neighbors list, update as needed
		// TODO: this should use channels to make it concurrency safe, like Add
		if neighbor, ok := ctx.Cluster.FindNeighborByUUID(r.FormValue("uuid")); ok {
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
			ctx.Cluster.UpdateNeighbor(*neighbor)
			ctx.SL.Info("updated existing neighbor")
			// TODO: gossip enable by accepting the list of neighbors
			// from the client and merging that data in.
			// for now, just let it update its own entry

		} else {
			// otherwise, add them to the Neighbors list
			ctx.SL.Info("adding neighbor")
			nd := NodeData{
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
			ctx.Cluster.AddNeighbor(nd)
		}
	}
	ar := AnnounceResponse{
		Nickname:  ctx.Cluster.Myself.Nickname,
		UUID:      ctx.Cluster.Myself.UUID,
		Location:  ctx.Cluster.Myself.Location,
		Writeable: ctx.Cluster.Myself.Writeable,
		BaseUrl:   ctx.Cluster.Myself.BaseUrl,
		Neighbors: ctx.Cluster.GetNeighbors(),
	}
	b, err := json.Marshal(ar)
	if err != nil {
		ctx.SL.Err(err.Error())
	}
	w.Write(b)
}

func FaviconHandler(w http.ResponseWriter, r *http.Request) {
	// just give it nothing to make it go away
	w.Write(nil)
}

var add_template = `
<html>
<head>
<title>{{.Title}}</title>
</head>

<body>
<h1>{{.Title}}</h1>

<form action="." method="post" enctype="multipart/form-data" >
{{if .RequireKey}}
<p>Upload key is required: <input type="text" name="key" /></p>
{{end}}
<input type="file" name="image" /><br />
initial sizes to pre-create: <input type="text" name="size_hints" /><br />
<input type="submit" value="upload image" />
</form>

</body>
</html>
`

var status_template = `
<html>
<head>
<title>{{.Title}}</title>
</head>

<body>
<h1>{{.Title}}</h1>

<h2>Config</h2>

<table>
	<tr><th>Port</th><td>{{ .Config.Port }}</td></tr>
	<tr><th>Replication</th><td>{{ .Config.Replication }}</td></tr>
	<tr><th>MinReplication</th><td>{{ .Config.MinReplication }}</td></tr>
	<tr><th>MaxReplication</th><td>{{ .Config.MaxReplication }}</td></tr>
	<tr><th># Resize Workers</th><td>{{ .Config.NumResizeWorkers }}</td></tr>
	<tr><th>Gossip sleep duration</th><td>{{ .Config.GossiperSleep }}</td></tr>
</table>

<h2>This Node</h2>

<table>
	<tr><th>Nickname</th><td>{{ .Cluster.Myself.Nickname }}</td></tr>
	<tr><th>UUID</th><td>{{ .Cluster.Myself.UUID }}</td></tr>
	<tr><th>Location</th><td>{{ .Cluster.Myself.Location }}</td></tr>
	<tr><th>Writeable</th><td>{{ .Cluster.Myself.Writeable }}</td></tr>
	<tr><th>Base URL</th><td>{{ .Cluster.Myself.BaseUrl }}</td></tr>
</table>

<h2>Neighbors</h2>

<table>
	<tr>
		<th>Nickname</th>
		<th>UUID</th>
		<th>BaseUrl</th>
		<th>Location</th>
		<th>Writeable</th>
		<th>LastSeen</th>
		<th>LastFailed</th>
	</tr>

{{ range .Neighbors }}

	<tr>
		<th>{{ .Nickname }}</th>
		<td>{{ .UUID }}</td>
		<td>{{ .BaseUrl }}</td>
		<td>{{ .Location }}</td>
		<td>{{ .Writeable }}</td>
		<td>{{ .LastSeen }}</td>
		<td>{{ .LastFailed }}</td>
	</tr>
	
{{ end }}

</table>


</body>
</html>
`
