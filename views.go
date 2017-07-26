package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/golang/groupcache"
	"github.com/thraxil/resize"
)

type context struct {
	cluster *cluster
	Cfg     siteConfig
	Ch      sharedChannels
	SL      log.Logger
}

type page struct {
	Title      string
	RequireKey bool
}

type imageData struct {
	Hash      string   `json:"hash"`
	Length    int      `json:"length"`
	Extension string   `json:"extension"`
	FullURL   string   `json:"full_url"`
	Satisfied bool     `json:"satisfied"`
	Nodes     []string `json:"nodes"`
}

func setCacheHeaders(w http.ResponseWriter, extension string) http.ResponseWriter {
	w.Header().Set("Content-Type", extmimes[extension[1:]])
	w.Header().Set("Expires", time.Now().Add(time.Hour*24*365).Format(time.RFC1123))
	return w
}

func parsePathServeImage(w http.ResponseWriter, r *http.Request,
	ctx context) (*imageSpecifier, bool) {
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) < 5) || (parts[1] != "image") {
		http.Error(w, "bad request", http.StatusNotFound)
		return nil, true
	}
	ahash, err := hashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "invalid hash", http.StatusNotFound)
		return nil, true
	}
	size := parts[3]
	if size == "" {
		http.Error(w, "missing size", http.StatusNotFound)
		return nil, true
	}
	s := resize.MakeSizeSpec(size)
	if s.String() != size {
		// force normalization of size spec
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+parts[4], http.StatusMovedPermanently)
		return nil, true
	}
	filename := parts[4]
	if filename == "" {
		filename = "image.jpg"
	}
	extension := filepath.Ext(filename)

	if extension == ".jpeg" {
		fixedFilename := strings.Replace(parts[4], ".jpeg", ".jpg", 1)
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+fixedFilename, http.StatusMovedPermanently)
		return nil, true
	}
	ri := &imageSpecifier{ahash, s, extension}
	return ri, false
}

func (ctx context) serveFromCluster(ri *imageSpecifier, w http.ResponseWriter) {
	// we don't have the full-size on this node either
	// need to check the rest of the cluster
	imgData, err := ctx.cluster.RetrieveImage(ri)
	if err != nil {
		// for now we just have to 404
		http.Error(w, "not found (serve from cluster)", http.StatusNotFound)
	} else {
		w = setCacheHeaders(w, ri.Extension)
		w.Write(imgData)
		servedFromCluster.Add(1)
	}
}

func (ctx context) serveDirect(ri *imageSpecifier, w http.ResponseWriter) bool {
	contents, err := ctx.Cfg.Backend.Read(*ri)
	if err == nil {
		// we've got it, so serve it directly
		w = setCacheHeaders(w, ri.Extension)
		w.Write(contents)
		servedLocally.Add(1)
		return true
	}
	return false
}

func serveImageHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	ri, handled := parsePathServeImage(w, r, ctx)
	if handled {
		return
	}
	var data []byte
	err := ctx.cluster.Imagecache.Get(nil, ri.String(),
		groupcache.AllocatingByteSliceSink(&data))
	if err == nil && len(data) > 0 {
		cacheHits.Add(1)
		w = setCacheHeaders(w, ri.Extension)
		w.Write(data)
		return
	}
	cacheMisses.Add(1)

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
		resizeFailures.Add(1)
		http.Error(w, "could not resize image", 500)
		return
	}
	if result.Magick {
		// imagemagick did the resize, so we just spit out
		// the sized file
		servedByMagick.Add(1)
		ctx.serveMagick(ri, w)
		return
	}
	servedScaled.Add(1)
	ctx.serveScaledByExtension(ri, w, *result.OutputImage)
}

func (ctx context) locallyWriteable() bool {
	return ctx.cluster.Myself.Writeable
}

func (ctx context) haveImageFullsizeLocally(ri *imageSpecifier) bool {
	_, err := ctx.Cfg.Backend.Read(ri.fullVersion())
	return err == nil
}

func (ctx context) serveScaledFromCluster(ri *imageSpecifier, w http.ResponseWriter) {
	imgData, err := ctx.cluster.RetrieveImage(ri)
	if err != nil {
		// for now we just have to 404
		http.Error(w, "not found (serveScaledFromCluster)", http.StatusNotFound)
	} else {
		servedFromCluster.Add(1)
		w = setCacheHeaders(w, ri.Extension)
		w.Write(imgData)
	}
	return
}

func (ctx context) makeResizeJob(ri *imageSpecifier) resizeResponse {
	c := make(chan resizeResponse)
	fmt.Println(ri.fullSizePath(ctx.Cfg.UploadDirectory))
	ctx.Ch.ResizeQueue <- resizeRequest{ri.fullSizePath(ctx.Cfg.UploadDirectory), ri.Extension, ri.Size.String(), c}
	resizeQueueLength.Add(1)
	result := <-c
	resizeQueueLength.Add(-1)
	return result
}

func (ctx context) serveMagick(ri *imageSpecifier, w http.ResponseWriter) {
	imgContents, err := ctx.Cfg.Backend.Read(*ri)
	if err != nil {
		ctx.SL.Log("level", "ERR", "msg", "couldn't read image resized by magick",
			"error", err)
		return
	}
	w = setCacheHeaders(w, ri.Extension)
	w.Write(imgContents)
}

func (ctx context) serveScaledByExtension(ri *imageSpecifier, w http.ResponseWriter,
	outputImage image.Image) {

	w = setCacheHeaders(w, ri.Extension)
	ctx.Cfg.Backend.writeLocalType(*ri, outputImage, extencoders[ri.Extension])
	serveType(w, outputImage, extencoders[ri.Extension])
}

func serveType(w http.ResponseWriter, outputImage image.Image, encFunc encfunc) {
	encFunc(w, outputImage)
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

func addHandler(w http.ResponseWriter, r *http.Request, ctx context) {
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
		ahash, err := hashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
		if err != nil {
			http.Error(w, "bad hash", 500)
			return
		}
		i.Seek(0, 0)
		mimetype := fh.Header.Get("Content-Type")
		if mimetype == "" {
			// they left off a mimetype, so default to jpg
			mimetype = "image/jpeg"
		}
		ext := mimeexts[mimetype]
		ri := imageSpecifier{
			ahash,
			resize.MakeSizeSpec("full"),
			"." + ext,
		}
		ctx.Cfg.Backend.WriteFull(ri, i)

		sizeHints := r.FormValue("size_hints")
		// yes, the full-size for this image gets written to disk on
		// this node even if it may not be one of the "right" ones
		// for it to end up on. This isn't optimal, but is easy
		// and we can just let the verify/balance worker clean it up
		// at some point in the future.

		// now stash it to other nodes in the cluster too
		nodes := ctx.cluster.Stash(ri, sizeHints, ctx.Cfg.Replication, ctx.Cfg.MinReplication, ctx.Cfg.Backend)
		id := imageData{
			Hash:      ahash.String(),
			Extension: ext,
			FullURL:   "/image/" + ahash.String() + "/full/image." + ext,
			Satisfied: len(nodes) >= ctx.Cfg.MinReplication,
			Nodes:     nodes,
		}
		b, err := json.Marshal(id)
		if err != nil {
			ctx.SL.Log("level", "ERR", "error", err.Error())
		}
		w.Write(b)
		ctx.cluster.Uploaded(imageRecord{*ahash, "." + ext})
	} else {
		p := page{
			Title:      "upload image",
			RequireKey: ctx.Cfg.KeyRequired(),
		}
		t, _ := template.New("add").Parse(addTemplate)
		t.Execute(w, &p)
	}
}

type statusPage struct {
	Title     string
	Config    siteConfig
	Cluster   *cluster
	Neighbors []nodeData
}

func statusHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	p := statusPage{
		Title:     "Status",
		Config:    ctx.Cfg,
		Cluster:   ctx.cluster,
		Neighbors: ctx.cluster.GetNeighbors(),
	}
	t, _ := template.New("status").Parse(statusTemplate)
	t.Execute(w, p)
}

type dashboardPage struct {
	RecentlyVerified []imageRecord
	RecentlyUploaded []imageRecord
	RecentlyStashed  []imageRecord
}

func dashboardHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	p := dashboardPage{
		RecentlyVerified: ctx.cluster.recentlyVerified,
		RecentlyUploaded: ctx.cluster.recentlyUploaded,
		RecentlyStashed:  ctx.cluster.recentlyStashed,
	}
	t, _ := template.New("dashboard").Parse(dashboardTemplate)
	t.Execute(w, p)
}

func configHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	b, err := json.Marshal(ctx.cluster.Myself)
	if err != nil {
		ctx.SL.Log("level", "ERR", "error", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func stashHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	n := ctx.cluster.Myself
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
	ahash, err := hashFromString(fmt.Sprintf("%x", h.Sum(nil)), "")
	if err != nil {
		http.Error(w, "bad hash", http.StatusNotFound)
		return
	}

	path := ctx.Cfg.UploadDirectory + ahash.AsPath()
	os.MkdirAll(path, 0755)
	ext := filepath.Ext(fh.Filename)
	fullpath := path + "/full" + ext
	f, _ := os.OpenFile(fullpath, os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()
	i.Seek(0, 0)
	io.Copy(f, i)
	fmt.Fprint(w, "ok")
	// do any eager resizing in the background
	sizeHints := r.FormValue("size_hints")
	go func() {
		sizes := strings.Split(sizeHints, ",")
		for _, size := range sizes {
			if size == "" {
				continue
			}
			c := make(chan resizeResponse)
			ctx.Ch.ResizeQueue <- resizeRequest{fullpath, ext, size, c}
			result := <-c
			if !result.Success {
				ctx.SL.Log("level", "ERR", "msg", "could not pre-resize")
			}
		}
	}()
	ctx.cluster.Stashed(imageRecord{*ahash, ext})
}

func retrieveInfoHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	// request will look like /retrieve_info/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve_info") {
		http.Error(w, "bad request", http.StatusNotFound)
		return
	}
	ahash, err := hashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "bad hash", http.StatusNotFound)
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
	n := ctx.cluster.Myself
	if size != "full" && !n.Writeable {
		// anything other than full-size, we can't do
		// if we don't have it already
		_, err = os.Open(baseDir + "/" + size + "." + extension)
		if err != nil {
			local = false
		}
	}

	b, err := json.Marshal(imageInfoResponse{ahash.String(), extension, local})
	if err != nil {
		ctx.SL.Log("level", "ERR", "error", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func retrieveHandler(w http.ResponseWriter, r *http.Request, ctx context) {

	// request will look like /retrieve/$hash/$size/$ext/
	parts := strings.Split(r.URL.String(), "/")
	if (len(parts) != 6) || (parts[1] != "retrieve") {
		http.Error(w, "bad request", http.StatusNotFound)
		return
	}
	ahash, err := hashFromString(parts[2], "")
	if err != nil {
		http.Error(w, "bad hash", http.StatusNotFound)
		return
	}
	size := parts[3]
	extension := parts[4]

	ri := imageSpecifier{
		ahash,
		resize.MakeSizeSpec(size),
		extension,
	}

	contents, err := ctx.Cfg.Backend.Read(ri)
	if err == nil {
		// we've got it, so serve it directly
		w.Header().Set("Content-Type", extmimes[extension])
		w.Write(contents)
		return
	}
	_, err = ctx.Cfg.Backend.Read(ri.fullVersion())
	if err != nil {
		// we don't have the full-size on this node either
		http.Error(w, "not found (retrieveHandler)", http.StatusNotFound)
		return
	}
	// we do have the full-size, but not the scaled one
	// so resize it, cache it, and serve it.

	// if we aren't writeable, we can't resize locally though.
	// 404 and let another node handle it
	n := ctx.cluster.Myself
	if !n.Writeable {
		http.Error(w, "could not resize image", http.StatusNotFound)
		return
	}

	c := make(chan resizeResponse)
	ctx.Ch.ResizeQueue <- resizeRequest{ri.fullSizePath(ctx.Cfg.UploadDirectory), "." + extension, size, c}
	result := <-c
	if !result.Success {
		http.Error(w, "could not resize image", 500)
		return
	}
	if result.Magick {
		// imagemagick did the resize, so we just spit out
		// the sized file
		w.Header().Set("Content-Type", extmimes[extension])
		imgContents, _ := ctx.Cfg.Backend.Read(ri)
		w.Write(imgContents)
		return
	}
	outputImage := *result.OutputImage

	w.Header().Set("Content-Type", extmimes[extension])
	ctx.Cfg.Backend.writeLocalType(ri, outputImage, extencoders[ri.Extension])
	serveType(w, outputImage, extencoders[ri.Extension])
}

func announceHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	if r.Method == "POST" {
		// another node is announcing themselves to us
		// if they are already in the Neighbors list, update as needed
		// TODO: this should use channels to make it concurrency safe, like Add
		if neighbor, ok := ctx.cluster.FindNeighborByUUID(r.FormValue("uuid")); ok {
			if r.FormValue("nickname") != "" {
				neighbor.Nickname = r.FormValue("nickname")
			}
			if r.FormValue("location") != "" {
				neighbor.Location = r.FormValue("location")
			}
			if r.FormValue("base_url") != "" {
				neighbor.BaseURL = r.FormValue("base_url")
			}
			if r.FormValue("writeable") != "" {
				neighbor.Writeable = r.FormValue("writeable") == "true"
			}
			neighbor.LastSeen = time.Now()
			ctx.cluster.UpdateNeighbor(*neighbor)
			ctx.SL.Log("level", "INFO", "msg", "updated existing neighbor")
			// TODO: gossip enable by accepting the list of neighbors
			// from the client and merging that data in.
			// for now, just let it update its own entry

		} else {
			// otherwise, add them to the Neighbors list
			ctx.SL.Log("level", "INFO", "msg", "adding neighbor")
			nd := nodeData{
				Nickname: r.FormValue("nickname"),
				UUID:     r.FormValue("uuid"),
				BaseURL:  r.FormValue("base_url"),
				Location: r.FormValue("location"),
			}
			if r.FormValue("writeable") == "true" {
				nd.Writeable = true
			} else {
				nd.Writeable = false
			}
			nd.LastSeen = time.Now()
			ctx.cluster.AddNeighbor(nd)
		}
	}
	ar := announceResponse{
		Nickname:  ctx.cluster.Myself.Nickname,
		UUID:      ctx.cluster.Myself.UUID,
		Location:  ctx.cluster.Myself.Location,
		Writeable: ctx.cluster.Myself.Writeable,
		BaseURL:   ctx.cluster.Myself.BaseURL,
		Neighbors: ctx.cluster.GetNeighbors(),
	}
	b, err := json.Marshal(ar)
	if err != nil {
		ctx.SL.Log("level", "ERR", "error", err.Error())
	}
	w.Write(b)
}

func joinHandler(w http.ResponseWriter, r *http.Request, ctx context) {
	if r.Method == "POST" {
		if r.FormValue("url") == "" {
			fmt.Fprint(w, "no url specified")
			return
		}
		url := r.FormValue("url")
		configURL := url + "/config/"
		res, err := http.Get(configURL)
		if err != nil {
			fmt.Fprint(w, "error retrieving config")
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Fprintf(w, "error reading body of response")
			return
		}
		var n nodeData
		err = json.Unmarshal(body, &n)
		if err != nil {
			fmt.Fprintf(w, "error parsing json")
			return
		}

		if n.UUID == ctx.cluster.Myself.UUID {
			fmt.Fprintf(w, "I can't join myself, silly!")
			return
		}
		_, ok := ctx.cluster.FindNeighborByUUID(n.UUID)
		if ok {
			fmt.Fprintf(w, "already have a node with that UUID in the cluster")
			// let's not do updates through this. Let gossip handle that.
			return
		}
		ctx.cluster.AddNeighbor(n)

		fmt.Fprintf(w, fmt.Sprintf("Added node %s [%s]", n.Nickname, n.UUID))

	} else {
		// show form
		w.Write([]byte(joinTemplate))
	}
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just give it nothing to make it go away
	w.Write(nil)
}

const joinTemplate = `
<html><head><title>Add Node</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>
<body>

<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
</ol>

<h1>Add Node</h1>
<form action="." method="post">
<input type="text" name="url" placeholder="Base URL" size="128" /><br />
<input type="submit" value="add node" />
</form>
</body>
</html>
`

const addTemplate = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>

<body>

<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
</ol>

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

const statusTemplate = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>

<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
</ol>

<body>
<div class="container">
<h1>Reticulum Node: {{ .Cluster.Myself.Nickname }}</h1>

<h2>Config</h2>

<table class="table">
	<tr><th>Port</th><td>{{ .Config.Port }}</td></tr>
	<tr><th>Replication</th><td>{{ .Config.Replication }}</td></tr>
	<tr><th>MinReplication</th><td>{{ .Config.MinReplication }}</td></tr>
	<tr><th>MaxReplication</th><td>{{ .Config.MaxReplication }}</td></tr>
	<tr><th># Resize Workers</th><td>{{ .Config.NumResizeWorkers }}</td></tr>
	<tr><th>Gossip sleep duration</th><td>{{ .Config.GossiperSleep }}</td></tr>
</table>

<h2>This Node</h2>

<table class="table">
	<tr><th>Nickname</th><td>{{ .Cluster.Myself.Nickname }}</td></tr>
	<tr><th>UUID</th><td>{{ .Cluster.Myself.UUID }}</td></tr>
	<tr><th>Location</th><td>{{ .Cluster.Myself.Location }}</td></tr>

	<tr><th>Writeable</th><td>{{if .Cluster.Myself.Writeable}}<span class="text-success">yes</span>{{else}}<span class="text-danger">read-only</span>{{end}}</td></tr>

	<tr><th>Base URL</th><td>{{ .Cluster.Myself.BaseURL }}</td></tr>
</table>

<h2>Neighbors</h2>

<table class="table table-condensed table-striped">
	<tr>
		<th>Nickname</th>
		<th>UUID</th>
		<th>BaseURL</th>
		<th>Location</th>
		<th>Writeable</th>
		<th>LastSeen</th>
		<th>LastFailed</th>
	</tr>

{{ range .Neighbors }}

	<tr>
		<th>{{ .Nickname }}</th>
		<td>{{ .UUID }}</td>
		<td><a href="http://{{.BaseURL}}">{{ .BaseURL }}</a>
        <div class="btn-group btn-group-sm" role="group">
        <a class="btn btn-default" href="http://{{.BaseURL}}/status/">S</a>
        <a class="btn btn-default" href="http://{{.BaseURL}}/dashboard/">D</a>
        <a class="btn btn-default" href="http://{{.BaseURL}}/debug/vars">E</a>
        </div>
    </td>
		<td>{{ .Location }}</td>
		<td>{{if .Writeable}}<span class="text-success">yes</span>{{else}}<span class="text-danger">read-only</span>{{end}}</td>
		<td>{{ if .LastSeen.IsZero}}-{{else}}{{ .LastSeenFormatted }}{{end}}</td>
		<td>{{ if .LastFailed.IsZero }}-{{else}}{{.LastFailedFormatted}}{{end}}</td>
	</tr>
	
{{ end }}

</table>
</div>

</body>
</html>
`

const dashboardTemplate = `
<html>
<head>
<title>Reticulum Dashboard</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
</head>

<body>
<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
</ol>


<div class="container">

<h2>Recently Verified</h2>

{{ range .RecentlyVerified }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}"></a>
{{ end }}

<h2>Recently Uploaded</h2>

{{ range .RecentlyUploaded }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}"></a>
{{ end }}

<h2>Recently Stashed</h2>

{{ range .RecentlyStashed }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}"></a>
{{ end }}


</div>

</body>
</html>
`
