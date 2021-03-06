package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
)

// what we know about a single node
// (ourself or another)
type nodeData struct {
	Nickname   string    `json:"nickname"`
	UUID       string    `json:"uuid"`
	BaseURL    string    `json:"base_url"`
	Location   string    `json:"location"`
	Writeable  bool      `json:"writeable"`
	LastSeen   time.Time `json:"last_seen"`
	LastFailed time.Time `json:"last_failed"`
}

// REPLICAS specifies how many times to duplicate each node entry in the ring
var REPLICAS = 16

func (n nodeData) String() string {
	return "Node - nickname: " + n.Nickname + " UUID: " + n.UUID
}

// RFC 8601 timestamp
func (n nodeData) LastSeenFormatted() string {
	return n.LastSeen.Format("2006-01-02 15:04:05")
}

// RFC 8601 timestamp
func (n nodeData) LastFailedFormatted() string {
	return n.LastFailed.Format("2006-01-02 15:04:05")
}

func (n nodeData) hashKeys() []string {
	keys := make([]string, REPLICAS)
	h := sha1.New()
	for i := range keys {
		h.Reset()
		io.WriteString(h, fmt.Sprintf("%s%d", n.UUID, i))
		keys[i] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return keys
}

func (n nodeData) IsCurrent() bool {
	return n.LastSeen.Unix() > n.LastFailed.Unix()
}

// returns version of the BaseURL that we know
// starts with 'http://' and does not end with '/'
func (n nodeData) goodBaseURL() string {
	url := n.BaseURL
	if !strings.HasPrefix(n.BaseURL, "http://") {
		url = "http://" + url
	}
	if strings.HasSuffix(url, "/") {
		url = url[:len(url)-1]
	}
	return url
}

func (n nodeData) retrieveURL(ri *imageSpecifier) string {
	return n.goodBaseURL() + ri.retrieveURLPath()
}

func (n nodeData) retrieveInfoURL(ri *imageSpecifier) string {
	return n.goodBaseURL() + ri.retrieveInfoURLPath()
}

func (n nodeData) stashURL() string {
	return n.goodBaseURL() + "/stash/"
}

func (n *nodeData) RetrieveImage(ctx context.Context, ri *imageSpecifier) ([]byte, error) {
	req, err := http.NewRequest("GET", n.retrieveURL(ri), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))

	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	} // otherwise, we got the image
	defer resp.Body.Close()
	n.LastSeen = time.Now()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	return ioutil.ReadAll(resp.Body)
}

type imageInfoResponse struct {
	Hash      string `json:"hash"`
	Extension string `json:"extension"`
	Local     bool   `json:"local"`
}

func (n *nodeData) RetrieveImageInfo(ctx context.Context, ri *imageSpecifier) (*imageInfoResponse, error) {
	url := n.retrieveInfoURL(ri)
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))

	if err != nil {
		n.LastFailed = time.Now()
		return nil, err
	}

	// otherwise, we got the info
	n.LastSeen = time.Now()
	return n.processRetrieveInfoResponse(resp)
}

func (n *nodeData) processRetrieveInfoResponse(resp *http.Response) (*imageInfoResponse, error) {
	if resp == nil {
		return nil, errors.New("nil response")
	}
	defer resp.Body.Close()
	if resp.Status != "200 OK" {
		return nil, errors.New("404, probably")
	}
	var response imageInfoResponse
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

func postFile(ctx context.Context, filename string, targetURL string, sizeHints string) (*http.Response, error) {
	bodyBuf := bytes.NewBufferString("")
	bodyWriter := multipart.NewWriter(bodyBuf)
	bodyWriter.WriteField("sizeHints", sizeHints)
	fileWriter, err := bodyWriter.CreateFormFile("image", filename)
	if err != nil {
		return nil, err
	}
	fh, err := os.Open(filename)
	if err != nil {
		bodyWriter.Close()
		return nil, err
	}
	defer fh.Close()
	io.Copy(fileWriter, fh)
	// .Close() finishes setting it up
	// do not defer this or it will make and empty POST request
	bodyWriter.Close()
	contentType := bodyWriter.FormDataContentType()
	req, err := http.NewRequest("POST", targetURL, bodyBuf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req.WithContext(ctx))
}

func (n *nodeData) Stash(ctx context.Context, ri imageSpecifier, sizeHints string, backend backend) bool {
	filename := backend.fullPath(ri)
	resp, err := postFile(ctx, filename, n.stashURL(), sizeHints)
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

func (n nodeData) announceURL() string {
	return n.goodBaseURL() + "/announce/"
}

type announceResponse struct {
	Nickname  string     `json:"nickname"`
	UUID      string     `json:"uuid"`
	Location  string     `json:"location"`
	Writeable bool       `json:"writeable"`
	BaseURL   string     `json:"base_url"`
	Neighbors []nodeData `json:"neighbors"`
}

type pingResponse struct {
	Resp *http.Response
	Err  error
}

func makeParams(originator nodeData) url.Values {
	params := url.Values{}
	params.Set("uuid", originator.UUID)
	params.Set("nickname", originator.Nickname)
	params.Set("location", originator.Location)
	params.Set("base_url", originator.BaseURL)
	if originator.Writeable {
		params.Set("writeable", "true")
	} else {
		params.Set("writeable", "false")
	}
	return params
}

func (n *nodeData) Ping(originator nodeData, sl log.Logger) (announceResponse, error) {
	params := makeParams(originator)

	var response announceResponse
	sl.Log("level", "INFO", "msg", n.announceURL())
	rc := make(chan pingResponse, 1)
	go func() {
		sl.Log("level", "INFO", "msg", "made request")
		resp, err := http.PostForm(n.announceURL(), params)
		rc <- pingResponse{resp, err}
	}()

	select {
	case pr := <-rc:
		resp := pr.Resp
		err := pr.Err
		if err != nil {
			sl.Log("level", "INFO", "msg", "node returned an error on ping", "node", n.Nickname, "error", err.Error())
			n.LastFailed = time.Now()
			return response, err
		}
		n.LastSeen = time.Now()
		// todo, update Writeable, Nickname, etc.
		b, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(b, &response)
		resp.Body.Close()
		if err != nil {
			sl.Log("level", "ERR", "bad json response", "value", fmt.Sprintf("%s", b))
			return response, errors.New("bad JSON response")
		}
		return response, nil

	case <-time.After(1 * time.Second):
		// if they take more than a second to respond
		// let's cut them out
		sl.Log("level", "ERR", "msg", "response timed out")
		n.LastFailed = time.Now()
		return response, errors.New("response timed out")
	}
}
