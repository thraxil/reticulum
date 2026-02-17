package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

func Test_LastSeenFormatted(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	if len(n.LastSeenFormatted()) != 19 {
		t.Error("looks like a badly formatted date")
	}
}

func Test_LastFailedFormatted(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	if len(n.LastFailedFormatted()) != 19 {
		t.Error("looks like a badly formatted date")
	}
}

func Test_hashkeys(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	keys := n.hashKeys()
	if len(keys) != REPLICAS {
		t.Error("not the right number of keys")
	}
	var expected = []string{
		"ae28605f0ffc34fe5314342f78efaa13ee45f699",
		"9affa344bca678572b044b50f4809e942389fbf6",
		"23360f51d95ce71902ea2b9b313de1f9c05c92a7",
		"8f6264d6b5b15840667d2414b7285a3fb7f63878",
		"e562b9e5dbfca62143230cd1e762005ffad74f8d",
		"7e212f8b753580f2e0bab7a234202c971be46626",
		"c426b974570120afd310fb9ece0c29c266f1738a",
		"9193bc5c3ae69a053fc7dc703b6b56cd7fe65637",
		"b9282ad8cc00462a1070e6ac7dab2c0867476f9c",
		"9a260dc2b8804efcd77f0b634b9bf258bef2b4ca",
		"07e025010da6c456e242d9d3d1075617aed1c4ff",
		"f70773bc3cb0b4d7084421c3389fc58e132c9852",
		"49cd9aa81076f95b02d2aa125d9fab1e62fa31cc",
		"88ca97909f7cdf94f201d6b90a265157067b3430",
		"8c37b2c35b1d5f4dcd878fe6a11f3b5a02ee62a2",
		"fb682e05b9be61797601e60165825c0b089f755e"}
	for i := range keys {
		if keys[i] != expected[i] {
			t.Error("bad key")
		}
	}
}

func testOneURL(n nodeData, ri *imageSpecifier, t *testing.T,
	retrieveURL, retrieveInfoURL, stashURL, announceURL string) {
	if n.retrieveURL(ri) != retrieveURL {
		t.Error("bad retrieve url")
	}
	if n.retrieveInfoURL(ri) != retrieveInfoURL {
		t.Error("bad retrieve info url")
	}
	if n.stashURL() != stashURL {
		t.Error("bad stash url")
	}
	if n.announceURL() != announceURL {
		t.Error("bad announce url")
	}
}

func Test_Urls(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	hash, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
	if err != nil {
		t.Error("bad hash")
	}
	s := resize.MakeSizeSpec("full")
	ri := &imageSpecifier{hash, s, "jpg"}

	testOneURL(n, ri, t,
		"http://localhost:8080/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8080/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8080/stash/",
		"http://localhost:8080/announce/",
	)

	n.BaseURL = "localhost:8080/"
	testOneURL(n, ri, t,
		"http://localhost:8080/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8080/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8080/stash/",
		"http://localhost:8080/announce/",
	)

	n.BaseURL = "http://localhost:8081/"
	testOneURL(n, ri, t,
		"http://localhost:8081/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8081/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8081/stash/",
		"http://localhost:8081/announce/",
	)

	n.BaseURL = "http://localhost:8081"
	testOneURL(n, ri, t,
		"http://localhost:8081/retrieve/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8081/retrieve_info/fb682e05b9be61797601e60165825c0b089f755e/full/jpg/",
		"http://localhost:8081/stash/",
		"http://localhost:8081/announce/",
	)
}

func Test_NodeString(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	if n.String() != "Node - nickname: test node UUID: test-uuid" {
		t.Error("wrong stringification")
	}
}

func Test_IsCurrent(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	if n.IsCurrent() {
		t.Error("should be equal for now")
	}
	n.LastSeen = time.Now()
	if !n.IsCurrent() {
		t.Error("should be current now")
	}
}

func Test_makeParams(t *testing.T) {
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	u := makeParams(n)
	if u.Get("uuid") != n.UUID {
		t.Error("couldn't make params")
	}
	n.Writeable = false
	u = makeParams(n)
	if u.Get("writeable") != "false" {
		t.Error("wrong boolean value")
	}
}

func TestStash(t *testing.T) {
	// Create a mock HTTP server that will act as the remote node.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the request is for the /stash/ endpoint.
		if r.URL.Path != "/stash/" {
			t.Errorf("Expected to request '/stash/', got: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Create a temporary file to act as the image to be stashed.
	tmpfile, err := os.CreateTemp("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	if _, err := tmpfile.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Create a nodeData instance with the mock server's URL as its BaseURL.
	n := nodeData{
		Nickname:  "test node",
		UUID:      "test-uuid",
		BaseURL:   server.URL,
		Location:  "test",
		Writeable: true,
	}

	// Create a backend that returns the temporary file's path.
	b := mockBackend{
		fullPathFunc: func(ri imageSpecifier) string {
			return tmpfile.Name()
		},
	}

	// Create an imageSpecifier.
	h, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
	if err != nil {
		t.Fatal(err)
	}
	s := resize.MakeSizeSpec("full")
	ri := imageSpecifier{h, s, "jpg"}

	// Call the Stash method.
	if !n.Stash(context.Background(), ri, "somesizehints", b) {
		t.Error("Stash returned false")
	}
}

func TestStashFailures(t *testing.T) {
	// Create a temporary file to act as the image to be stashed.
	tmpfile, err := os.CreateTemp("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	if _, err := tmpfile.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Create an imageSpecifier.
	h, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
	if err != nil {
		t.Fatal(err)
	}
	s := resize.MakeSizeSpec("full")
	ri := imageSpecifier{h, s, "jpg"}

	tests := []struct {
		name       string
		server     *httptest.Server
		backend    backend
		expectFail bool
	}{
		{
			name: "postFile returns an error",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			})),
			backend: mockBackend{
				fullPathFunc: func(ri imageSpecifier) string {
					return "/non-existent-file"
				},
			},
			expectFail: true,
		},
		{
			name: "resp.StatusCode != 200",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})),
			backend: mockBackend{
				fullPathFunc: func(ri imageSpecifier) string {
					return tmpfile.Name()
				},
			},
			expectFail: true,
		},
		{
			name: "io.ReadAll returns an error",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "10")
			})),
			backend: mockBackend{
				fullPathFunc: func(ri imageSpecifier) string {
					return tmpfile.Name()
				},
			},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.server.Close()
			n := nodeData{
				BaseURL: tt.server.URL,
			}
			if tt.expectFail {
				if n.Stash(context.Background(), ri, "somesizehints", tt.backend) {
					t.Error("Expected Stash to fail, but it succeeded")
				}
			} else {
				if !n.Stash(context.Background(), ri, "somesizehints", tt.backend) {
					t.Error("Expected Stash to succeed, but it failed")
				}
			}
		})
	}
}

type mockReadCloser struct {
	io.Reader
}

func (m *mockReadCloser) Close() error {
	return errors.New("close error")
}

func TestRetrieveImage(t *testing.T) {
	tests := []struct {
		name          string
		server        *httptest.Server
		url           string
		expectSuccess bool
	}{
		{
			name: "successful retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("image data"))
			})),
			expectSuccess: true,
		},
		{
			name: "failed retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})),
			expectSuccess: false,
		},
		{
			name:          "bad url",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			url:           "http://\177", // invalid control character
			expectSuccess: false,
		},
		{
			name: "close error",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write([]byte("image data"))
			})),
			expectSuccess: false,
		},
		{
			name:          "do error",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			url:           "http://bad.url",
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// we need to handle this case separately because we need to inject the mockReadCloser
			if tt.name == "close error" {
				serv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "10")
					_, _ = w.Write([]byte("aaaaaaaaaa"))
				}))
				defer serv.Close()
				// Create an imageSpecifier.
				h, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
				if err != nil {
					t.Fatal(err)
				}
				s := resize.MakeSizeSpec("full")
				ri := &imageSpecifier{h, s, "jpg"}

				// Create a nodeData.
				n := nodeData{BaseURL: serv.URL}

				// need to manually do the request here to inject the mock closer
				req, err := http.NewRequest("GET", n.retrieveURL(ri), nil)
				if err != nil {
					t.Fatal(err)
				}
				resp, err := http.DefaultClient.Do(req.WithContext(context.Background()))
				if err != nil {
					t.Fatal(err)
				}
				resp.Body = &mockReadCloser{resp.Body}
				_, err = n.processRetrieveImageResponse(resp)
				if err == nil {
					t.Error("expected an error from processRetrieveImageResponse")
				}
				return
			}

			defer tt.server.Close()

			// Create an imageSpecifier.
			h, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
			if err != nil {
				t.Fatal(err)
			}
			s := resize.MakeSizeSpec("full")
			ri := &imageSpecifier{h, s, "jpg"}

			// Create a nodeData.
			n := nodeData{}
			if tt.url != "" {
				n.BaseURL = tt.url
			} else {
				n.BaseURL = tt.server.URL
			}

			// Retrieve the image from the node.
			img, err := n.RetrieveImage(context.Background(), ri)

			if tt.expectSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if string(img) != "image data" {
					t.Errorf("Expected image data to be 'image data', but got '%s'", string(img))
				}
			} else {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			}
		})
	}
}

func TestRetrieveImageInfo(t *testing.T) {
	tests := []struct {
		name          string
		server        *httptest.Server
		url           string
		nilResponse   bool
		expectSuccess bool
	}{
		{
			name: "successful retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"hash": "testhash", "extension": "jpg", "local": true}`))
			})),
			expectSuccess: true,
		},
		{
			name: "failed retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})),
			expectSuccess: false,
		},
		{
			name:          "bad url",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			url:           "http://\177", // invalid control character
			expectSuccess: false,
		},
		{
			name: "bad json",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				badJSON := `{"hash": "testhash"`
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(badJSON)))
				_, _ = w.Write([]byte(badJSON))
			})),
			expectSuccess: false,
		},
		{
			name: "io.ReadAll error",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "10")
			})),
			expectSuccess: false,
		},
		{
			name:          "nil response",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			nilResponse:   true,
			expectSuccess: false,
		},
		{
			name:          "do error",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			url:           "http://bad.url",
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.server.Close()

			// Create an imageSpecifier.
			h, err := hashFromString("fb682e05b9be61797601e60165825c0b089f755e", "")
			if err != nil {
				t.Fatal(err)
			}
			s := resize.MakeSizeSpec("full")
			ri := &imageSpecifier{h, s, "jpg"}

			// Create a nodeData.
			n := nodeData{}
			if tt.url != "" {
				n.BaseURL = tt.url
			} else {
				n.BaseURL = tt.server.URL
			}

			if tt.nilResponse {
				// Test processRetrieveInfoResponse with a nil response.
				_, err := n.processRetrieveInfoResponse(nil)
				if err == nil {
					t.Error("Expected an error for nil response, but got none")
				}
				return
			}

			// Retrieve the image info from the node.
			info, err := n.RetrieveImageInfo(context.Background(), ri)

			if tt.expectSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if info.Hash != "testhash" {
					t.Errorf("Expected info.Hash to be 'testhash', but got '%s'", info.Hash)
				}
			} else {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			}
		})
	}
}

func TestPing(t *testing.T) {

	tests := []struct {
		name string

		server *httptest.Server

		expectSuccess bool
	}{

		{

			name: "successful ping",

			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				_, _ = w.Write([]byte(`{"nickname": "test-neighbor"}`))

			})),

			expectSuccess: true,
		},

		{

			name: "server error",

			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				w.WriteHeader(http.StatusInternalServerError)

			})),

			expectSuccess: false,
		},

		{

			name: "bad json",

			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				_, _ = w.Write([]byte(`{"nickname":`))

			})),

			expectSuccess: false,
		},

		{

			name: "timeout",

			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				time.Sleep(2 * time.Second)

			})),

			expectSuccess: false,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			defer tt.server.Close()

			logger := log.NewNopLogger()

			n := nodeData{BaseURL: tt.server.URL}

			originator := nodeData{}

			_, err := n.Ping(originator, logger)

			if tt.expectSuccess {

				if err != nil {

					t.Fatal(err)

				}

			} else {

				if err == nil {

					t.Error("Expected an error, but got none")

				}

			}

		})

	}

}

func TestPostFile(t *testing.T) {
	tests := []struct {
		name          string
		server        *httptest.Server
		filename      string
		badURL        bool
		expectSuccess bool
	}{
		{
			name: "successful post",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			})),
			expectSuccess: true,
		},
		{
			name:          "bad url",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			badURL:        true,
			expectSuccess: false,
		},
		{
			name:     "file not found",
			server:   httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			filename: "/non-existent-file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.server.Close()

			filename := tt.filename
			if filename == "" {
				tmpfile, err := os.CreateTemp("", "example")
				if err != nil {
					t.Fatal(err)
				}
				filename = tmpfile.Name()
				defer func() { _ = os.Remove(filename) }()
				if _, err := tmpfile.Write([]byte("hello")); err != nil {
					t.Fatal(err)
				}
				if err := tmpfile.Close(); err != nil {
					t.Fatal(err)
				}
			}

			url := tt.server.URL
			if tt.badURL {
				url = "http://\177"
			}

			_, err := postFile(context.Background(), filename, url, "somesizehints")

			if tt.expectSuccess {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Error("Expected an error, but got none")
				}
			}
		})
	}
}
