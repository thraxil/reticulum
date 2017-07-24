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
