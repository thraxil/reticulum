package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"

	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

type sitecontext struct {
	cluster          Cluster
	Cfg              *siteConfig
	Ch               sharedChannels
	SL               log.Logger
	ImageView        *ImageView
	UploadView       *UploadView
	StashView        *StashView
	RetrieveInfoView *RetrieveInfoView
	RetrieveView     *RetrieveView
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
	w.Header().Set("Content-Type", extmimes[extension])
	w.Header().Set("Expires", time.Now().Add(time.Hour*24*365).Format(time.RFC1123))
	return w
}

func parsePathServeImage(w http.ResponseWriter, r *http.Request,
	ctx sitecontext) (*imageSpecifier, bool) {
	hash := r.PathValue("hash")
	size := r.PathValue("size")
	filename := r.PathValue("filename")

	ahash, err := hashFromString(hash, "")
	if err != nil {
		http.Error(w, "invalid hash", http.StatusNotFound)
		return nil, true
	}
	if size == "" {
		http.Error(w, "missing size", http.StatusNotFound)
		return nil, true
	}
	s := resize.MakeSizeSpec(size)
	if s.String() != size {
		// force normalization of size spec
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+filename, http.StatusMovedPermanently)
		return nil, true
	}
	if filename == "" {
		filename = "image.jpg"
	}
	extension := filepath.Ext(filename)

	if extension == ".jpeg" {
		fixedFilename := strings.Replace(filename, ".jpeg", ".jpg", 1)
		http.Redirect(w, r, "/image/"+ahash.String()+"/"+s.String()+"/"+fixedFilename, http.StatusMovedPermanently)
		return nil, true
	}
	ri := &imageSpecifier{ahash, s, extension}
	return ri, false
}

func serveImageHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	if r.PathValue("size") == "debug" {
		debugImageHandler(w, r, ctx)
		return
	}
	ri, handled := parsePathServeImage(w, r, ctx)
	if handled {
		return
	}

	imgData, etag, err := ctx.ImageView.GetImage(r.Context(), ri)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound) // Use 404 for not found errors
		return
	}

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w = setCacheHeaders(w, ri.Extension)
	w.Header().Set("Etag", etag)
	_, _ = w.Write(imgData)
	servedLocally.Add(1) // Assuming if GetImage succeeds, it was served eventually
}

type debugNodeInfo struct {
	Node       nodeData
	ShouldHave bool
	HasIt      bool
	Status     string
	IsMyself   bool
}

type debugPage struct {
	Title     string
	Hash      string
	Thumbnail string
	Nodes     []debugNodeInfo
}

func debugImageHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	hash := r.PathValue("hash")
	filename := r.PathValue("filename")
	ahash, err := hashFromString(hash, "")
	if err != nil {
		http.Error(w, "invalid hash", http.StatusNotFound)
		return
	}

	allNodes := ctx.cluster.NeighborsInclusive()
	writeOrder := ctx.cluster.WriteOrder(hash)
	replication := ctx.Cfg.Replication

	shouldHaveMap := make(map[string]bool)
	for i, n := range writeOrder {
		if i < replication {
			shouldHaveMap[n.UUID] = true
		}
	}

	var infos []debugNodeInfo
	// using "full" size to check existence of the original image
	ri := &imageSpecifier{ahash, resize.MakeSizeSpec("full"), filepath.Ext(filename)}

	for i := range allNodes {
		n := &allNodes[i]
		shouldHave := shouldHaveMap[n.UUID]
		hasIt := false
		var status string

		// check if node has it
		// use a short timeout
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		info, err := n.RetrieveImageInfo(checkCtx, ri)
		cancel()

		if err != nil {
			status = "error: " + err.Error()
		} else {
			if info.Local {
				hasIt = true
				status = "ok"
			} else {
				status = "missing"
			}
		}

		infos = append(infos, debugNodeInfo{
			Node:       *n,
			ShouldHave: shouldHave,
			HasIt:      hasIt,
			Status:     status,
			IsMyself:   n.UUID == ctx.cluster.GetMyself().UUID,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].ShouldHave != infos[j].ShouldHave {
			return infos[i].ShouldHave
		}
		return infos[i].Node.Nickname < infos[j].Node.Nickname
	})

	thumbnail := "/image/" + ahash.String() + "/100s/" + filename

	p := debugPage{
		Title:     "Debug Info",
		Hash:      hash,
		Thumbnail: thumbnail,
		Nodes:     infos,
	}
	t, _ := template.New("debug").Parse(debugTemplate)
	_ = t.Execute(w, p)
}

const debugTemplate = `
<html>
<head>
<title>{{.Title}}</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
<style>
.node-row { }
.should-have-yes { background-color: #e6f3ff; }
.has-it-yes { color: green; font-weight: bold; }
.has-it-no { color: red; }
.status-error { color: orange; }
</style>
</head>
<body>

<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
  <li><a href="/logs/">Logs</a></li>
  <li class="active">Image Debug</li>
</ol>

<div class="container">
<h1>Image Debug: {{.Hash}}</h1>
<a href="/image/{{.Hash}}/full/image.jpg"><img src="{{.Thumbnail}}" /></a>

<table class="table table-bordered">
<thead>
<tr>
    <th>Node</th>
    <th>UUID</th>
    <th>Writeable</th>
    <th>Should Have?</th>
    <th>Has It?</th>
    <th>Status</th>
</tr>
</thead>
<tbody>
{{ range .Nodes }}
<tr class="node-row {{if .ShouldHave}}should-have-yes{{end}}">
    <td><a href="{{.Node.BaseURL}}">{{.Node.Nickname}}</a>{{if .IsMyself}} <strong>(this node)</strong>{{end}}</td>
    <td>{{.Node.UUID}}</td>
    <td>{{if .Node.Writeable}}yes{{else}}no{{end}}</td>
    <td>{{if .ShouldHave}}YES{{else}}no{{end}}</td>
    <td class="{{if .HasIt}}has-it-yes{{else}}has-it-no{{end}}">{{if .HasIt}}YES{{else}}NO{{end}}</td>
    <td class="{{if ne .Status "ok"}}status-error{{end}}">{{.Status}}</td>
</tr>
{{ end }}
</tbody>
</table>

</div>
</body>
</html>
`

var mimeexts = map[string]string{
	"image/jpeg": "jpg",
	"image/gif":  "gif",
	"image/png":  "png",
	"image/webp": "webp",
}

var extmimes = map[string]string{
	".jpg":  "image/jpeg",
	".gif":  "image/gif",
	".png":  "image/png",
	".webp": "image/webp",
}

func getAddHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	p := page{
		Title:      "upload image",
		RequireKey: ctx.Cfg.KeyRequired(),
	}
	t, _ := template.New("add").Parse(addTemplate)
	_ = t.Execute(w, &p)
}

func postAddHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	key := r.FormValue("key")
	sizeHints := r.FormValue("size_hints")

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image file", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// Use io.ReadSeeker for imageFile
	imageFile, ok := file.(io.ReadSeeker)
	if !ok {
		http.Error(w, "image file does not implement io.ReadSeeker", http.StatusInternalServerError)
		return
	}

	responseBytes, err := ctx.UploadView.UploadImage(r.Context(), key, imageFile, fileHeader, sizeHints)
	if err != nil {
		if strings.Contains(err.Error(), "invalid upload key") {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else if strings.Contains(err.Error(), "unsupported image type") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "bad hash") {
			http.Error(w, err.Error(), http.StatusInternalServerError) // Or BadRequest depending on source of bad hash
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseBytes)
}

type statusPage struct {
	Title     string
	Config    siteConfig
	Cluster   Cluster
	Neighbors []nodeData
}

func statusHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	p := statusPage{
		Title:     "Status",
		Config:    *ctx.Cfg,
		Cluster:   ctx.cluster,
		Neighbors: ctx.cluster.GetNeighbors(),
	}
	t, _ := template.New("status").Parse(statusTemplate)
	_ = t.Execute(w, p)
}

type dashboardPage struct {
	RecentlyVerified []imageRecord
	RecentlyUploaded []imageRecord
	RecentlyStashed  []imageRecord
}

func reverseImages(images []imageRecord) []imageRecord {
	newImages := make([]imageRecord, len(images))
	for i, img := range images {
		newImages[len(images)-1-i] = img
	}
	return newImages
}

func dashboardHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	p := dashboardPage{
		RecentlyVerified: reverseImages(ctx.cluster.GetRecentlyVerified()),
		RecentlyUploaded: reverseImages(ctx.cluster.GetRecentlyUploaded()),
		RecentlyStashed:  reverseImages(ctx.cluster.GetRecentlyStashed()),
	}
	t, _ := template.New("dashboard").Parse(dashboardTemplate)
	_ = t.Execute(w, p)
}

func configHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	b, err := json.Marshal(ctx.cluster.GetMyself())
	if err != nil {
		_ = ctx.SL.Log("level", "ERR", "error", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func stashHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	sizeHints := r.FormValue("size_hints")

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "no image uploaded", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// Use io.ReadSeeker for imageFile
	imageFile, ok := file.(io.ReadSeeker)
	if !ok {
		http.Error(w, "image file does not implement io.ReadSeeker", http.StatusInternalServerError)
		return
	}

	response, err := ctx.StashView.StashImage(r.Context(), imageFile, fileHeader, sizeHints)
	if err != nil {
		if strings.Contains(err.Error(), "non-writeable node") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "unsupported image type") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "bad hash") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	_, _ = fmt.Fprint(w, response)
}

func retrieveInfoHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	hash := r.PathValue("hash")
	size := r.PathValue("size") // Note: size is used for writeable check in GetImageInfo, not directly here
	ext := r.PathValue("ext")

	responseBytes, err := ctx.RetrieveInfoView.GetImageInfo(hash, size, ext)
	if err != nil {
		if strings.Contains(err.Error(), "bad hash") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseBytes)
}

func retrieveHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	hash := r.PathValue("hash")
	size := r.PathValue("size")
	ext := r.PathValue("ext")
	ifNoneMatch := r.Header.Get("If-None-Match")

	imgData, etag, err := ctx.RetrieveView.RetrieveImage(r.Context(), hash, size, ext, ifNoneMatch)
	if err != nil {
		// Specific error handling for different scenarios can be added here
		// For now, a generic 404 for not found and 500 for other errors.
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if ifNoneMatch != "" && ifNoneMatch == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", extmimes["."+ext])
	w.Header().Set("Etag", etag)
	_, _ = w.Write(imgData)
}

func getAnnounceHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	ar := announceResponse{
		Nickname:  ctx.cluster.GetMyself().Nickname,
		UUID:      ctx.cluster.GetMyself().UUID,
		Location:  ctx.cluster.GetMyself().Location,
		Writeable: ctx.cluster.GetMyself().Writeable,
		BaseURL:   ctx.cluster.GetMyself().BaseURL,
		Neighbors: ctx.cluster.GetNeighbors(),
	}
	b, err := json.Marshal(ar)
	if err != nil {
		_ = ctx.SL.Log("level", "ERR", "error", err.Error())
	}
	_, _ = w.Write(b)
}

func postAnnounceHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
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
		_ = ctx.SL.Log("level", "INFO", "msg", "updated existing neighbor")
		// TODO: gossip enable by accepting the list of neighbors
		// from the client and merging that data in.
		// for now, just let it update its own entry

	} else {
		// otherwise, add them to the Neighbors list
		_ = ctx.SL.Log("level", "INFO", "msg", "adding neighbor")
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
	getAnnounceHandler(w, r, ctx)
}
func getJoinHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	// show form
	_, _ = w.Write([]byte(joinTemplate))
}

func postJoinHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	if r.FormValue("url") == "" {
		_, _ = fmt.Fprint(w, "no url specified")
		return
	}
	url := r.FormValue("url")
	configURL := url + "/config/"
	rctx := r.Context()
	req, err := http.NewRequest("GET", configURL, nil)
	if err != nil {
		_, _ = fmt.Fprintf(w, "bad config URL")
		return
	}
	res, err := http.DefaultClient.Do(req.WithContext(rctx))
	if err != nil {
		_, _ = fmt.Fprint(w, "error retrieving config")
		return
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		_, _ = fmt.Fprintf(w, "error reading body of response")
		return
	}
	var n nodeData
	err = json.Unmarshal(body, &n)
	if err != nil {
		_, _ = fmt.Fprintf(w, "error parsing json")
		return
	}

	if n.UUID == ctx.cluster.GetMyself().UUID {
		_, _ = fmt.Fprintf(w, "I can't join myself, silly!")
		return
	}
	_, ok := ctx.cluster.FindNeighborByUUID(n.UUID)
	if ok {
		_, _ = fmt.Fprintf(w, "already have a node with that UUID in the cluster")
		// let's not do updates through this. Let gossip handle that.
		return
	}
	ctx.cluster.AddNeighbor(n)

	_, _ = fmt.Fprintf(w, "Added node %s [%s]", n.Nickname, n.UUID)
}

type logsPage struct {
	Logs []LogEntry
}

func logsHandler(w http.ResponseWriter, r *http.Request, ctx sitecontext) {
	p := logsPage{
		Logs: GlobalLogCache.StructuredEntries(),
	}
	t, _ := template.New("logs").Parse(logsTemplate)
	_ = t.Execute(w, p)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	// just give it nothing to make it go away
	_, _ = w.Write(nil)
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
  <li><a href="/logs/">Logs</a></li>
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
  <li><a href="/logs/">Logs</a></li>
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
  <li><a href="/logs/">Logs</a></li>
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
  <li><a href="/logs/">Logs</a></li>
</ol>


<div class="container">

<h2>Recently Verified</h2>

{{ range .RecentlyVerified }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}" width="100" height="100"></a>
{{ end }}

<h2>Recently Uploaded</h2>

{{ range .RecentlyUploaded }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}" width="100" height="100"></a>
{{ end }}

<h2>Recently Stashed</h2>

{{ range .RecentlyStashed }}
<a href="/image/{{.Hash.String}}/full/image{{.Extension}}"><img src="/image/{{ .Hash.String }}/100s/image{{.Extension}}" width="100" height="100"></a>
{{ end }}


</div>

</body>
</html>
`

const logsTemplate = `
<html>
<head>
<title>Reticulum Logs</title>
<link rel="stylesheet" href="//maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css" />
<style>
.log-table td { font-family: monospace; font-size: 0.9em; }
.level-ERR { color: red; font-weight: bold; }
.level-WARN { color: orange; font-weight: bold; }
.level-INFO { color: green; }
.log-details { padding-left: 2em !important; background-color: #f9f9f9; }
</style>
</head>
<body>

<ol class="breadcrumb">
  <li><a href="/">Upload</a></li>
  <li><a href="/status/">Status</a></li>
  <li><a href="/dashboard/">Dashboard</a></li>
  <li><a href="/debug/vars">expvar</a></li>
  <li><a href="/join/">Add Node</a></li>
  <li class="active">Logs</li>
</ol>

<div class="container-fluid">
<h1>Logs</h1>
<table class="table table-condensed log-table">
<thead>
<tr>
    <th>Time</th>
    <th>Level</th>
    <th>Comp.</th>
    <th>Node</th>
    <th>Message</th>
    <th>Method</th>
    <th>Path</th>
    <th>Dur.</th>
    <th>Remote</th>
</tr>
</thead>
<tbody>
{{ range .Logs }}
<tr>
    <td>{{ .Timestamp }}</td>
    <td class="level-{{.Level}}">{{ .Level }}</td>
    <td>{{ .Component }}</td>
    <td>{{ .Node }}</td>
    <td>{{ .Message }}</td>
    <td>{{ .Method }}</td>
    <td>{{ .Path }}</td>
    <td>{{ .Duration }}</td>
    <td>{{ .RemoteAddr }}</td>
</tr>
{{ if or .Error .Image .Replication }}
<tr>
    <td colspan="9" class="log-details">
        {{ if .Error }}<div><strong>Error:</strong> {{ .Error }}</div>{{ end }}
        {{ if .Image }}<div><strong>Image:</strong> {{ .Image }}</div>{{ end }}
        {{ if .Replication }}<div><strong>Repl:</strong> {{ .Replication }}</div>{{ end }}
    </td>
</tr>
{{ end }}
{{ end }}
</tbody>
</table>
</div>
</body>
</html>
`
