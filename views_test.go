package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeTestContext() sitecontext {
	var n []nodeData
	_, c := makeNewClusterData(n)
	b := newDiskBackend("")
	cfg := siteConfig{Backend: b}
	return sitecontext{cluster: c, Cfg: cfg}
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
	ctx := makeTestContext()

	cases := []serveImageHandlerTestCase{
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpg", http.StatusNotFound},
		{"/foo", http.StatusNotFound},
		{"/image/invalidahash/full/image.jpg", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22//image.jpg", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/", http.StatusNotFound},
		{"/image/0051ec03fb813e8731224ee06feee7c828ceae22/100s/image.jpeg", http.StatusMovedPermanently},
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
		// TODO: this one should not be OK
		{"/retrieve_info/0051ec03fb813e8731224ee06feee7c828ceae22//jpg/", http.StatusOK},
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

func Test_RetrieveImageHandler(t *testing.T) {
	ctx := makeTestContext()

	cases := []serveImageHandlerTestCase{
		{"/retrieve/0051ec03fb813e8731224ee06feee7c828ceae22/100s/jpg/", http.StatusNotFound},
		{"/foo", http.StatusNotFound},
		{"/retrieve/invalidahash/full/jpg/", http.StatusNotFound},
		// TODO: this one should not be OK
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
