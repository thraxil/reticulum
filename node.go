package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"
)

// what we know about a single node
// (ourself or another)
type NodeData struct {
	Nickname      string    `json:"nickname"`
	UUID          string    `json:"uuid"`
	BaseUrl       string    `json:"base_url"`
	GroupcacheUrl string    `json:"groupcache_url"`
	Location      string    `json:"location"`
	Writeable     bool      `json:"writeable"`
	LastSeen      time.Time `json:"last_seen"`
	LastFailed    time.Time `json:"last_failed"`
}

var REPLICAS = 16

func (n NodeData) String() string {
	return "Node - nickname: " + n.Nickname + " UUID: " + n.UUID
}

func (n NodeData) HashKeys() []string {
	keys := make([]string, REPLICAS)
	h := sha1.New()
	for i := range keys {
		h.Reset()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return keys
}

func (n NodeData) IsCurrent() bool {
	return n.LastSeen.Unix() > n.LastFailed.Unix()
}

// I come from Python, what can I say?
func startswith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func endswith(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

// returns version of the BaseUrl that we know
// starts with 'http://' and does not end with '/'
func (n NodeData) goodBaseUrl() string {
	url := n.BaseUrl
	if !startswith(n.BaseUrl, "http://") {
		url = "http://" + url
	}
	if endswith(url, "/") {
		url = url[:len(url)-1]
	}
	return url
}

func (n NodeData) retrieveUrl(ri *ImageSpecifier) string {
	return n.goodBaseUrl() + ri.retrieveUrlPath()
}

func (n NodeData) retrieveInfoUrl(ri *ImageSpecifier) string {
	return n.goodBaseUrl() + ri.retrieveInfoUrlPath()
}

func (n NodeData) stashUrl() string {
	return n.goodBaseUrl() + "/stash/"
}

func (n *NodeData) RetrieveImage(ri *ImageSpecifier) ([]byte, error) {
	resp, err := http.Get(n.retrieveUrl(ri))
	defer resp.Body.Close()
	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	} // otherwise, we got the image
	n.LastSeen = time.Now()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	b, _ := ioutil.ReadAll(resp.Body)
	return b, nil
}

type ImageInfoResponse struct {
	Hash      string `json:"hash"`
	Extension string `json:"extension"`
	Local     bool   `json:"local"`
}

func timedGetRequest(url string, duration time.Duration) (resp *http.Response, err error) {
	rc := make(chan pingResponse, 1)
	go func() {
		resp, err := http.Get(url)
		rc <- pingResponse{resp, err}
	}()
	select {
	case pr := <-rc:
		resp = pr.Resp
		err = pr.Err
	case <-time.After(duration):
		err = errors.New("GET request timed out")
	}
	return
}

func (n *NodeData) RetrieveImageInfo(ri *ImageSpecifier) (*ImageInfoResponse, error) {
	url := n.retrieveInfoUrl(ri)
	resp, err := timedGetRequest(url, 1*time.Second)
	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	}

	// otherwise, we got the info
	n.LastSeen = time.Now()
	return n.processRetrieveInfoResponse(resp)
}

func (n *NodeData) processRetrieveInfoResponse(resp *http.Response) (*ImageInfoResponse, error) {
	if resp == nil {
		return nil, errors.New("nil response")
	}
	defer resp.Body.Close()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	var response ImageInfoResponse
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func postFile(filename string, target_url string, size_hints string) (*http.Response, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	body_writer.WriteField("size_hints", size_hints)
	file_writer, err := body_writer.CreateFormFile("image", filename)
	if err != nil {
		panic(err.Error())
	}
	fh, err := os.Open(filename)
	if err != nil {
		body_writer.Close()
		return nil, err
	}
	defer fh.Close()
	io.Copy(file_writer, fh)
	// .Close() finishes setting it up
	// do not defer this or it will make and empty POST request
	body_writer.Close()
	content_type := body_writer.FormDataContentType()
	return http.Post(target_url, content_type, body_buf)
}

func (n *NodeData) Stash(filename string, size_hints string) bool {
	resp, err := postFile(filename, n.stashUrl(), size_hints)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	b, _ := ioutil.ReadAll(resp.Body)
	return string(b) == "ok"
}

func (n NodeData) announceUrl() string {
	return n.goodBaseUrl() + "/announce/"
}

type AnnounceResponse struct {
	Nickname      string     `json:"nickname"`
	UUID          string     `json:"uuid"`
	Location      string     `json:"location"`
	Writeable     bool       `json:"writeable"`
	BaseUrl       string     `json:"base_url"`
	GroupcacheUrl string     `json:"groupcache_url"`
	Neighbors     []NodeData `json:"neighbors"`
}

type pingResponse struct {
	Resp *http.Response
	Err  error
}

func makeParams(originator NodeData) url.Values {
	params := url.Values{}
	params.Set("uuid", originator.UUID)
	params.Set("nickname", originator.Nickname)
	params.Set("location", originator.Location)
	params.Set("base_url", originator.BaseUrl)
	params.Set("groupcache_url", originator.GroupcacheUrl)
	if originator.Writeable {
		params.Set("writeable", "true")
	} else {
		params.Set("writeable", "false")
	}
	return params
}

func (n *NodeData) Ping(originator NodeData) (AnnounceResponse, error) {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	params := makeParams(originator)

	var response AnnounceResponse
	sl.Info(n.announceUrl())
	rc := make(chan pingResponse, 1)
	go func() {
		sl.Info("made request")
		resp, err := http.PostForm(n.announceUrl(), params)
		rc <- pingResponse{resp, err}
	}()

	select {
	case pr := <-rc:
		resp := pr.Resp
		err = pr.Err
		if err != nil {
			sl.Info(fmt.Sprintf("node %s returned an error on ping: %s", n.Nickname, err.Error()))
			n.LastFailed = time.Now()
			return response, err
		} else {
			n.LastSeen = time.Now()
			// todo, update Writeable, Nickname, etc.
			b, _ := ioutil.ReadAll(resp.Body)
			err = json.Unmarshal(b, &response)
			resp.Body.Close()
			if err != nil {
				sl.Err("bad json response")
				sl.Err(fmt.Sprintf("%s", b))
				return response, errors.New("bad JSON response")
			}
			return response, nil
		}
	case <-time.After(1 * time.Second):
		// if they take more than a second to respond
		// let's cut them out
		sl.Err("response timed out")
		n.LastFailed = time.Now()
		return response, errors.New("response timed out")
	}
}
