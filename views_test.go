package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_StatusHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/status/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}
	rec := httptest.NewRecorder()
	StatusHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_DashboardHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/dashboard/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}
	rec := httptest.NewRecorder()
	DashboardHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_AddFormHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}
	rec := httptest.NewRecorder()
	AddHandler(rec, req, ctx)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.Status)
	}
}

func Test_FaviconHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "localhost:8080/favicon.ico", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	rec := httptest.NewRecorder()
	FaviconHandler(rec, req)

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
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}

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

type ServeImageHandlerTestCase struct {
	path   string
	status int
}

func Test_ServeImageHandler(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}

	cases := []ServeImageHandlerTestCase{
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
		rec := httptest.NewRecorder()
		ServeImageHandler(rec, req, ctx)

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}

type RetreiveInfoHandlerTestCase struct {
	path   string
	status int
}

func Test_RetrieveInfoImageHandler(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	ctx := Context{Cluster: c}

	cases := []ServeImageHandlerTestCase{
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
		rec := httptest.NewRecorder()
		RetrieveInfoHandler(rec, req, ctx)

		res := rec.Result()
		if res.StatusCode != c.status {
			t.Errorf("for %s expected status %v; got %v", c.path, c.status, res.Status)
		}
	}
}
