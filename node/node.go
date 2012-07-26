package node

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
	Nickname   string    `json:"nickname"`
	UUID       string    `json:"uuid"`
	BaseUrl    string    `json:"base_url"`
	Location   string    `json:"location"`
	Writeable  bool      `json:"bool"`
	LastSeen   time.Time `json:"last_seen"`
	LastFailed time.Time `json:"last_failed"`
}

var REPLICAS = 16

func (n NodeData) String() string {
	return "Node - nickname: " + n.Nickname + " UUID: " + n.UUID
}

func (n NodeData) HashKeys() []string {
	keys := make([]string, REPLICAS)
	for i := range keys {
		h := sha1.New()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = string(fmt.Sprintf("%x", h.Sum(nil)))
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

func (n NodeData) retrieveUrl(hash string, size string, extension string) string {
	return n.goodBaseUrl() + "/retrieve/" + hash + "/" + size + "/" + extension + "/"
}

func (n NodeData) retrieveInfoUrl(hash string, size string, extension string) string {
	return n.goodBaseUrl() + "/retrieve_info/" + hash + "/" + size + "/" + extension + "/"
}

func (n NodeData) stashUrl() string {
	return n.goodBaseUrl() + "/stash/"
}

func (n *NodeData) RetrieveImage(hash string, size string, extension string) ([]byte, error) {
	resp, err := http.Get(n.retrieveUrl(hash, size, extension))
	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	} // otherwise, we got the image
	n.LastSeen = time.Now()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return b, nil
}

type ImageInfoResponse struct {
	Hash      string `json:"hash"`
	Extension string `json:"extension"`
	Local     bool   `json:"local"`
}

func (n *NodeData) RetrieveImageInfo(hash string, size string, extension string) (*ImageInfoResponse, error) {
	resp, err := http.Get(n.retrieveInfoUrl(hash, size, extension))
	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	}
	// otherwise, we got the info
	n.LastSeen = time.Now()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	var response ImageInfoResponse
	b, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	err = json.Unmarshal(b, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func postFile(filename string, target_url string) (*http.Response, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	file_writer, err := body_writer.CreateFormFile("image", filename)
	if err != nil {
		panic(err.Error())
	}
	fh, err := os.Open(filename)
	if err != nil {
		panic(err.Error())
	}
	io.Copy(file_writer, fh)
	content_type := body_writer.FormDataContentType()
	body_writer.Close()
	return http.Post(target_url, content_type, body_buf)
}

func (n *NodeData) Stash(filename string) bool {
	_, err := postFile(filename, n.stashUrl())
	if err != nil {
		// this node failed us, so take them out of
		// the write ring until we hear otherwise from them
		// TODO: look more closely at the response to 
		//       possibly act differently in specific cases
		//       ie, allow them to specify a temporary failure
		n.LastFailed = time.Now()
		n.Writeable = false
	} else {
		n.LastSeen = time.Now()
	}
	return err == nil
}

func (n NodeData) announceUrl() string {
	return "http://" + n.BaseUrl + "/announce/"
}

type AnnounceResponse struct {
	Nickname  string     `json:"nickname"`
	UUID      string     `json:"uuid"`
	Location  string     `json:"location"`
	Writeable bool       `json:"writeable"`
	BaseUrl   string     `json:"base_url"`
	Neighbors []NodeData `json:"neighbors"`
}

func (n *NodeData) Ping(originator NodeData) (AnnounceResponse, error) {
	sl, err := syslog.New(syslog.LOG_INFO, "reticulum")
	if err != nil {
		log.Fatal("couldn't log to syslog")
	}
	params := url.Values{}
	params.Set("uuid", originator.UUID)
	params.Set("nickname", originator.Nickname)
	params.Set("location", originator.Location)
	params.Set("base_url", originator.BaseUrl)
	if originator.Writeable {
		params.Set("writeable", "true")
	} else {
		params.Set("writeable", "false")
	}

	var response AnnounceResponse
	sl.Info(n.announceUrl())
	resp, err := http.PostForm(n.announceUrl(), params)
	sl.Info("made request")
	if err != nil {
		sl.Info(fmt.Sprintf("node %s returned an error on ping: %s", n.Nickname, err.Error()))
		n.LastFailed = time.Now()
		return response, err
	} else {
		n.LastSeen = time.Now()
		// todo, update Writeable, Nickname, etc.
	}
	b, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	err = json.Unmarshal(b, &response)
	if err != nil {
		sl.Err("bad json response")
		sl.Err(fmt.Sprintf("%s", b))
	}
	return response, nil
}
