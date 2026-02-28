package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

func makeTestContextWithUploadDir(uploadDir string) sitecontext {
	var n []nodeData
	_, c := makeNewClusterData(n)
	b := newDiskBackend(uploadDir)
	cfg := siteConfig{Backend: b, UploadDirectory: uploadDir, Replication: 1, MinReplication: 1}
	ch := sharedChannels{
		ResizeQueue: make(chan resizeRequest),
	}
	sl := log.NewNopLogger()
	imageView := NewImageView(c, b, &cfg, ch, sl)
	uploadView := NewUploadView(c, b, &cfg, ch, sl)
	stashView := NewStashView(c, b, &cfg, ch, sl)
	retrieveInfoView := NewRetrieveInfoView(c, &cfg, sl)
	retrieveView := NewRetrieveView(imageView, sl)

	go func() {
		for req := range ch.ResizeQueue {
			img := image.NewRGBA(image.Rect(0, 0, 100, 100))
			var i image.Image = img
			req.Response <- resizeResponse{Success: true, OutputImage: &i}
		}
	}()

	return sitecontext{
		cluster:          c,
		Cfg:              &cfg,
		Ch:               ch,
		SL:               sl,
		ImageView:        imageView,
		UploadView:       uploadView,
		StashView:        stashView,
		RetrieveInfoView: retrieveInfoView,
		RetrieveView:     retrieveView,
	}
}

func makeTestContext() sitecontext {
	return makeTestContextWithUploadDir("")
}

func Test_statusHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/status/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	statusHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_dashboardHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/dashboard/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	dashboardHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_GetAddFormHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	getAddHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_faviconHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/favicon.ico", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	rec := httptest.NewRecorder()
	faviconHandler(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

type parsePathServeImageTestCase struct {
	path   string
	status int
	served bool
	size   string
}

func Test_parsePathServeImage(t *testing.T) {
	ctx := makeTestContext()

	cases := []parsePathServeImageTestCase{
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpg", http.StatusOK, false, "100s"},
		{"/foo", http.StatusNotFound, true, ""},
		{"/image/invalidahash/full/image.jpg", http.StatusNotFound, true, ""},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22//image.jpg", http.StatusNotFound, true, ""},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/", http.StatusOK, false, "100s"},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpeg", http.StatusMovedPermanently, true, ""},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.webp", http.StatusOK, false, "100s"},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", "localhost:8080"+c.path, nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}
		if c.status != http.StatusNotFound {
			parts := strings.Split(c.path, "/")
			if len(parts) > 4 {
				req.SetPathValue("hash", parts[2])
				req.SetPathValue("size", parts[3])
				req.SetPathValue("filename", parts[4])
			}
		}

		rec := httptest.NewRecorder()
		spec, served := parsePathServeImage(rec, req, ctx)

		if served != c.served {
			t.Errorf("%s served: %v != %v", c.path, c.served, served)
		}
		if !served && c.size != spec.Size.String() {
			t.Errorf("bad size spec")
		}

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}

type serveImageHandlerTestCase struct {
	path   string
	status int
}

func Test_serveImageHandler(t *testing.T) {
	ctx := makeTestContextWithUploadDir("test/uploads1/")

	cases := []serveImageHandlerTestCase{
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpg", http.StatusNotFound},
		{"/foo", http.StatusNotFound},
		{"/image/invalidahash/full/image.jpg", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22//image.jpg", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpeg", http.StatusMovedPermanently},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.webp", http.StatusOK},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", "localhost:8080"+c.path, nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}
		if c.status != http.StatusNotFound {
			parts := strings.Split(c.path, "/")
			req.SetPathValue("hash", parts[2])
			req.SetPathValue("size", parts[3])
			req.SetPathValue("filename", parts[4])
		}
		rec := httptest.NewRecorder()
		serveImageHandler(rec, req, ctx)

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}

func Test_RetrieveInfoImageHandler(t *testing.T) {
	ctx := makeTestContext()

	cases := []serveImageHandlerTestCase{
		{"/retrieve_info/0051ec03fb813e8731224ee06feee7c828ceae22/100s/jpg/", http.StatusOK},
		{"/foo", http.StatusNotFound},
		{"/retrieve_info/invalidahash/full/jpg/", http.StatusNotFound},
		{"/retrieve_info/0051ec03fb813e8731224ee06feee7c828ceae22//jpg/", http.StatusNotFound},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", "localhost:8080"+c.path, nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}
		if c.status != http.StatusNotFound {
			parts := strings.Split(c.path, "/")
			req.SetPathValue("hash", parts[2])
			req.SetPathValue("size", parts[3])
			req.SetPathValue("ext", parts[4])
		}
		rec := httptest.NewRecorder()
		retrieveInfoHandler(rec, req, ctx)

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}

func Test_serveImageHandler_direct(t *testing.T) {
	ctx := makeTestContextWithUploadDir("test/uploads1/")
	hash := "0051ec03fb813e8731224ee06feee7c828ceae22"
	ahash, _ := hashFromString(hash, "")
	ri := imageSpecifier{ahash, resize.MakeSizeSpec("100s"), ".webp"}
	// create a dummy file
	f, _ := os.Create(ri.sizedPath(ctx.Cfg.UploadDirectory))
	_ = f.Close()

	req, err := http.NewRequest("GET", "localhost:8080/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.webp", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.SetPathValue("hash", "0051ec03fb813e8731224ee06feee7c828ceae22")
	req.SetPathValue("size", "100s")
	req.SetPathValue("filename", "image.webp")
	rec := httptest.NewRecorder()
	serveImageHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("for /image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.webp expected status %v; got %v", http.StatusOK, res.Status)
	}
}

func Test_serveImageHandler_resize(t *testing.T) {
	ctx := makeTestContextWithUploadDir("test/uploads4/")
	ctx.cluster.Myself.Writeable = true
	hash := "c1986af3c26609b8b7d8933f99c51c1a89e9ea6b"
	ahash, _ := hashFromString(hash, "")
	ri := imageSpecifier{ahash, resize.MakeSizeSpec("full"), ".png"}
	// create a dummy file
	_ = os.MkdirAll(ri.baseDir(ctx.Cfg.UploadDirectory), 0755)
	f, _ := os.Create(ri.fullSizePath(ctx.Cfg.UploadDirectory))
	_ = f.Close()

	req, err := http.NewRequest("GET", "localhost:8080/image/c1986af3c26609b8b7d8933f99c51c1a89e9ea6b/100s/image.png", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.SetPathValue("hash", "c1986af3c26609b8b7d8933f99c51c1a89e9ea6b")
	req.SetPathValue("size", "100s")
	req.SetPathValue("filename", "image.png")
	rec := httptest.NewRecorder()
	serveImageHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("for /image/c1986af3c26609b8b7d8933f99c51c1a89e9ea6b/100s/image.png expected status %v; got %v", http.StatusOK, res.Status)
	}
}

func Test_RetrieveImageHandler(t *testing.T) {
	ctx := makeTestContext()

	cases := []serveImageHandlerTestCase{
		{"/retrieve/0051ec03fb813e8731224ee06feee7c828ceae22/100s/jpg/", http.StatusNotFound},
		{"/foo", http.StatusNotFound},
		{"/retrieve/invalidahash/full/jpg/", http.StatusNotFound},
		{"/retrieve/0051ec03fb813e8731224ee06feee7c828ceae22//jpg/", http.StatusNotFound},
	}
	for _, c := range cases {
		req, err := http.NewRequest("GET", "localhost:8080"+c.path, nil)
		if err != nil {
			t.Fatalf("could not create request: %v", err)
		}
		if c.status != http.StatusNotFound {
			parts := strings.Split(c.path, "/")
			req.SetPathValue("hash", parts[2])
			req.SetPathValue("size", parts[3])
			req.SetPathValue("ext", parts[4])
		}
		rec := httptest.NewRecorder()
		retrieveHandler(rec, req, ctx)

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}

func Test_retrieveHandler_found(t *testing.T) {
	ctx := makeTestContextWithUploadDir("test/uploads5/")
	hash := "c1986af3c26609b8b7d8933f99c51c1a89e9ea6b"
	ahash, _ := hashFromString(hash, "")
	ri := imageSpecifier{ahash, resize.MakeSizeSpec("100s"), ".png"}
	// create a dummy file
	_ = os.MkdirAll(ri.baseDir(ctx.Cfg.UploadDirectory), 0755)
	f, _ := os.Create(ri.sizedPath(ctx.Cfg.UploadDirectory))
	_ = f.Close()

	req, err := http.NewRequest("GET", "localhost:8080/retrieve/c1986af3c26609b8b7d8933f99c51c1a89e9ea6b/100s/png/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.SetPathValue("hash", "c1986af3c26609b8b7d8933f99c51c1a89e9ea6b")
	req.SetPathValue("size", "100s")
	req.SetPathValue("ext", "png")
	rec := httptest.NewRecorder()
	retrieveHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("for /retrieve/c1986af3c26609b8b7d8933f99c51c1a89e9ea6b/100s/png/ expected status %v; got %v", http.StatusOK, res.Status)
	}
}

func Test_configHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	configHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_GetAnnounceHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	getAnnounceHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_PostAddHandler(t *testing.T) {
	// Create a new multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"image", "gopher.png"))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("could not create form file: %v", err)
	}

	// Open the test image file
	f, err := os.Open("test/gopher.png")
	if err != nil {
		t.Fatalf("could not open test image: %v", err)
	}
	defer func() { _ = f.Close() }()

	// Copy the image data to the form file
	_, err = io.Copy(fw, f)
	if err != nil {
		t.Fatalf("could not copy image data: %v", err)
	}

	// Add the upload key to the form
	err = w.WriteField("key", "test-key")
	if err != nil {
		t.Fatalf("could not write key field: %v", err)
	}

	// Close the multipart writer
	err = w.Close()
	if err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/", &b)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads2/")
	ctx.Cfg.UploadKeys = []string{"test-key"}

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postAddHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Check the response body
	var data imageData
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		t.Fatalf("could not decode response body: %v", err)
	}

	// Check the image data
	if data.Hash != "c1986af3c26609b8b7d8933f99c51c1a89e9ea6b" {
		t.Errorf("unexpected hash: %s", data.Hash)
	}
	if data.Extension != "png" {
		t.Errorf("unexpected extension: %s", data.Extension)
	}
}

func Test_PostAddHandler_invalidKey(t *testing.T) {
	// Create a new multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"image", "gopher.png"))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("could not create form file: %v", err)
	}

	// Open the test image file
	f, err := os.Open("test/gopher.png")
	if err != nil {
		t.Fatalf("could not open test image: %v", err)
	}
	defer func() { _ = f.Close() }()

	// Copy the image data to the form file
	_, err = io.Copy(fw, f)
	if err != nil {
		t.Fatalf("could not copy image data: %v", err)
	}

	// Add the upload key to the form
	err = w.WriteField("key", "invalid-key")
	if err != nil {
		t.Fatalf("could not write key field: %v", err)
	}

	// Close the multipart writer
	err = w.Close()
	if err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/", &b)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads2/")
	ctx.Cfg.UploadKeys = []string{"test-key"}

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postAddHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden; got %v", res.Status)
	}
}

func Test_PostAddHandler_NotImage(t *testing.T) {
	// Create a new multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"image", "not_an_image.txt"))
	h.Set("Content-Type", "text/plain")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("could not create form file: %v", err)
	}

	_, err = fw.Write([]byte("this is not an image"))
	if err != nil {
		t.Fatalf("could not write to form file: %v", err)
	}

	// Add the upload key to the form
	err = w.WriteField("key", "test-key")
	if err != nil {
		t.Fatalf("could not write key field: %v", err)
	}

	// Close the multipart writer
	err = w.Close()
	if err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/", &b)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads2/")
	ctx.Cfg.UploadKeys = []string{"test-key"}

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postAddHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status Bad Request; got %v", res.Status)
	}
}

func Test_StashHandler_NoFile(t *testing.T) {
	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/stash/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads3/")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	stashHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status Bad Request; got %v", res.Status)
	}
}

func Test_StashHandler(t *testing.T) {
	// Create a new multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"image", "gopher.png"))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("could not create form file: %v", err)
	}

	// Open the test image file
	f, err := os.Open("test/gopher.png")
	if err != nil {
		t.Fatalf("could not open test image: %v", err)
	}
	defer func() { _ = f.Close() }()

	// Copy the image data to the form file
	_, err = io.Copy(fw, f)
	if err != nil {
		t.Fatalf("could not copy image data: %v", err)
	}

	// Close the multipart writer
	_ = w.Close()

	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/stash/", &b)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads3/")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	stashHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Calculate the expected hash
	hSHA1 := sha1.New()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("could not seek file: %v", err)
	}
	if _, err := io.Copy(hSHA1, f); err != nil {
		t.Fatalf("could not calculate hash: %v", err)
	}
	expectedHash, err := hashFromString(fmt.Sprintf("%x", hSHA1.Sum(nil)), "")
	if err != nil {
		t.Fatalf("could not create hash from string: %v", err)
	}
	expectedPath := "test/uploads3/" + expectedHash.AsPath() + "/full.png"

	// Check that the file was created
	_, err = os.Stat(expectedPath)
	if err != nil {
		t.Errorf("expected file to be created at %s: %v", expectedPath, err)
	}
}

func Test_StashHandler_NotImage(t *testing.T) {
	// Create a new multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			"image", "not_an_image.txt"))
	h.Set("Content-Type", "text/plain")
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("could not create form file: %v", err)
	}

	_, err = fw.Write([]byte("this is not an image"))
	if err != nil {
		t.Fatalf("could not write to form file: %v", err)
	}

	// Close the multipart writer
	err = w.Close()
	if err != nil {
		t.Fatalf("could not close multipart writer: %v", err)
	}
	// Create a new request with the multipart form data
	req, err := http.NewRequest("POST", "localhost:8080/stash/", &b)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Create a new test context
	ctx := makeTestContextWithUploadDir("test/uploads3/")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	stashHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status Bad Request; got %v", res.Status)
	}
}

func Test_PostJoinHandler(t *testing.T) {
	// Create a new test server to simulate the other node
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"nickname":"test-neighbor","uuid":"neighbor-uuid","base_url":"neighbor.example.com","location":"neighbor-location","writeable":true}`)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create a new test context
	ctx := makeTestContext()

	// Create a new request with the join data
	form := url.Values{}
	form.Add("url", server.URL)
	req, err := http.NewRequest("POST", "localhost:8080/join/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postJoinHandler(rec, req, ctx)

	// wait for the cluster to process the message
	ctx.cluster.Sync()

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Check that the neighbor was added
	_, ok := ctx.cluster.FindNeighborByUUID("neighbor-uuid")
	if !ok {
		t.Errorf("expected neighbor to be added")
	}
}

func Test_PostJoinHandler_AlreadyExists(t *testing.T) {
	// Create a new test server to simulate the other node
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"nickname":"test-neighbor","uuid":"neighbor-uuid","base_url":"neighbor.example.com","location":"neighbor-location","writeable":true}`)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create a new test context
	ctx := makeTestContext()

	// Add the neighbor to the cluster
	ctx.cluster.AddNeighbor(nodeData{
		Nickname: "test-neighbor",
		UUID:     "neighbor-uuid",
		BaseURL:  "neighbor.example.com",
		Location: "neighbor-location",
	})
	ctx.cluster.Sync()

	// Create a new request with the join data
	form := url.Values{}
	form.Add("url", server.URL)
	req, err := http.NewRequest("POST", "localhost:8080/join/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postJoinHandler(rec, req, ctx)

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Check that the neighbor was not added again
	if len(ctx.cluster.GetNeighbors()) != 1 {
		t.Errorf("expected neighbor to not be added again")
	}
}

func Test_GetJoinHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	ctx := makeTestContext()
	rec := httptest.NewRecorder()
	getJoinHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_PostAnnounceHandler_NewNeighbor(t *testing.T) {
	// Create a new test context
	ctx := makeTestContext()

	// Create a new request with the announce data
	form := url.Values{}
	form.Add("nickname", "test-neighbor")
	form.Add("uuid", "neighbor-uuid")
	form.Add("base_url", "neighbor.example.com")
	form.Add("location", "neighbor-location")
	form.Add("writeable", "true")

	req, err := http.NewRequest("POST", "localhost:8080/announce/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postAnnounceHandler(rec, req, ctx)

	// wait for the cluster to process the message
	ctx.cluster.Sync()

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Check that the neighbor was added
	neighbor, ok := ctx.cluster.FindNeighborByUUID("neighbor-uuid")
	if !ok {
		t.Errorf("expected neighbor to be added")
	}

	if neighbor.Nickname != "test-neighbor" {
		t.Errorf("wrong nickname")
	}
	if neighbor.BaseURL != "neighbor.example.com" {
		t.Errorf("wrong base_url")
	}
	if neighbor.Location != "neighbor-location" {
		t.Errorf("wrong location")
	}
	if !neighbor.Writeable {
		t.Errorf("wrong writeable")
	}
}

func Test_PostAnnounceHandler_UpdateNeighbor(t *testing.T) {
	// Create a new test context
	ctx := makeTestContext()

	// add a neighbor
	nd := nodeData{
		Nickname: "old-nickname",
		UUID:     "neighbor-uuid",
		BaseURL:  "old.example.com",
		Location: "old-location",
	}
	ctx.cluster.AddNeighbor(nd)

	// Create a new request with the announce data
	form := url.Values{}
	form.Add("nickname", "new-nickname")
	form.Add("uuid", "neighbor-uuid")
	form.Add("base_url", "new.example.com")
	form.Add("location", "new-location")
	form.Add("writeable", "true")

	req, err := http.NewRequest("POST", "localhost:8080/announce/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a new response recorder
	rec := httptest.NewRecorder()

	// Call the handler
	postAnnounceHandler(rec, req, ctx)

	// wait for the cluster to process the message
	ctx.cluster.Sync()

	// Check the response status code
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}

	// Check that the neighbor was updated
	neighbor, ok := ctx.cluster.FindNeighborByUUID("neighbor-uuid")
	if !ok {
		t.Errorf("expected neighbor to be present")
	}

	if neighbor.Nickname != "new-nickname" {
		t.Errorf("wrong nickname")
	}
	if neighbor.BaseURL != "new.example.com" {
		t.Errorf("wrong base_url")
	}
	if neighbor.Location != "new-location" {
		t.Errorf("wrong location")
	}
	if !neighbor.Writeable {
		t.Errorf("wrong writeable")
	}
}
