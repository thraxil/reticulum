package main

import (
	"context"

	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/thraxil/resize"
)

func Test_ClusterOfOneInitialNeighbors(t *testing.T) {
	var n []nodeData
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
}

func Test_ClusterOfOneNeighborsInclusive(t *testing.T) {
	var n []nodeData
	myself, c := makeNewClusterData(n)
	neighbors := c.NeighborsInclusive()
	if len(neighbors) != 1 {
		t.Error("too many neighbors for empty cluster")
	}
	if neighbors[0].Nickname != myself.Nickname {
		t.Error("single node is not myself")
	}
}

func Test_ClusterOfOneFindNeighbors(t *testing.T) {
	var n []nodeData
	myself, c := makeNewClusterData(n)

	_, found := c.FindNeighborByUUID("test-uuid")
	if found {
		t.Error("neighbors should be empty")
	}

	neighbors := c.WriteableNeighbors()
	if len(neighbors) != 1 {
		t.Error("too many neighbors for empty cluster")
	}
	if neighbors[0].Nickname != myself.Nickname {
		t.Error("single node is not myself")
	}

	r := c.Ring()
	if len(r) != REPLICAS {
		t.Error("wrong number of ring entries")
	}

	wr := c.WriteRing()
	if len(wr) != REPLICAS {
		t.Error("wrong number of write ring entries")
	}

	ro := c.ReadOrder("anyhash")
	if len(ro) != 1 {
		t.Error("only one node, should only be one result in the list")
	}
	if ro[0].UUID != myself.UUID {
		t.Error("it's not me!")
	}

	wo := c.WriteOrder("anyhash")
	if len(wo) != 1 {
		t.Error("only one node, should only be one result in the list")
	}
	if wo[0].UUID != myself.UUID {
		t.Error("it's not me!")
	}
}

func Test_AddNeighbor(t *testing.T) {
	var n []nodeData
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	c.AddNeighbor(nodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseURL:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	})
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}

}

func Test_RemoveNeighbor(t *testing.T) {
	var n []nodeData
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := nodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseURL:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	nd2 := nodeData{
		Nickname:  "addedneighbor3",
		UUID:      "test-uuid-3",
		BaseURL:   "localhost:8082",
		Location:  "test",
		Writeable: true,
	}
	c.AddNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}
	c.AddNeighbor(nd2)
	if len(c.GetNeighbors()) != 2 {
		t.Error("should be two")
	}
	c.RemoveNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be one in there")
	}
	c.RemoveNeighbor(nd2)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should be back to zero")
	}
}

func Test_UpdateNeighbor(t *testing.T) {
	var n []nodeData
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := nodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseURL:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	c.AddNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}
	nd.BaseURL = "localhost:8082"
	c.UpdateNeighbor(nd)
	neighbors := c.GetNeighbors()
	if neighbors[0].BaseURL != "localhost:8082" {
		t.Error("update didn't take")
	}
}

func Test_FailedNeighbor(t *testing.T) {
	var n []nodeData
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := nodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseURL:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	c.AddNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}
	nd.BaseURL = "localhost:8082"
	c.FailedNeighbor(nd)
	neighbors := c.GetNeighbors()
	if neighbors[0].Writeable {
		t.Error("failed notification didn't take")
	}
}

func TestClusterStash(t *testing.T) {
	tests := []struct {
		name          string
		server        *httptest.Server
		replication   int
		expectSuccess bool
	}{
		{
			name: "successful stash",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			})),
			replication:   1,
			expectSuccess: true,
		},
		{
			name: "failed stash",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})),
			replication:   1,
			expectSuccess: false,
		},
		{
			name: "multiple successful stashes",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			})),
			replication:   3,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.server.Close()

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

			// Create a backend.
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

			// Create a cluster.
			_, c := makeNewClusterData([]nodeData{})
			neighbor1 := nodeData{
				Nickname:  "neighbor1",
				UUID:      "neighbor1-uuid",
				BaseURL:   tt.server.URL,
				Writeable: true,
			}
			c.AddNeighbor(neighbor1)
			neighbor2 := nodeData{
				Nickname:  "neighbor2",
				UUID:      "neighbor2-uuid",
				BaseURL:   tt.server.URL,
				Writeable: true,
			}
			c.AddNeighbor(neighbor2)
			neighbor3 := nodeData{
				Nickname:  "neighbor3",
				UUID:      "neighbor3-uuid",
				BaseURL:   tt.server.URL,
				Writeable: true,
			}
			c.AddNeighbor(neighbor3)

			// Stash to the cluster.
			savedTo := c.Stash(context.Background(), ri, "", tt.replication, tt.replication, b)

			if tt.expectSuccess {
				// Check that the stash was successful.
				numSaved := 0
				for _, s := range savedTo {
					if s != "" {
						numSaved++
					}
				}
				if numSaved != tt.replication {
					t.Errorf("Expected to save to %d nodes, but saved to %d", tt.replication, numSaved)
				}
			} else {
				// Check that the stash failed.
				for _, s := range savedTo {
					if s != "" {
						t.Errorf("Expected to fail to save, but saved to %s", s)
					}
				}
			}
		})
	}
}

func TestClusterVerified(t *testing.T) {
	_, c := makeNewClusterData([]nodeData{})
	ir := imageRecord{}
	for i := 0; i < 21; i++ {
		c.verified(ir)
	}

	rc := make(chan []imageRecord)
	c.chF <- func() {
		rc <- c.recentlyVerified
	}
	rv := <-rc

	if len(rv) != 20 {
		t.Errorf("Expected recentlyVerified to have 20 elements, but it has %d", len(rv))
	}
}

func TestClusterUploaded(t *testing.T) {
	_, c := makeNewClusterData([]nodeData{})
	ir := imageRecord{}
	for i := 0; i < 21; i++ {
		c.Uploaded(ir)
	}

	rc := make(chan []imageRecord)
	c.chF <- func() {
		rc <- c.recentlyUploaded
	}
	rv := <-rc

	if len(rv) != 20 {
		t.Errorf("Expected recentlyUploaded to have 20 elements, but it has %d", len(rv))
	}
}

func TestClusterStashed(t *testing.T) {
	_, c := makeNewClusterData([]nodeData{})
	ir := imageRecord{}
	for i := 0; i < 21; i++ {
		c.Stashed(ir)
	}

	rc := make(chan []imageRecord)
	c.chF <- func() {
		rc <- c.recentlyStashed
	}
	rv := <-rc

	if len(rv) != 20 {
		t.Errorf("Expected recentlyStashed to have 20 elements, but it has %d", len(rv))
	}
}

func TestClusterRetrieveImage(t *testing.T) {
	tests := []struct {
		name          string
		server        *httptest.Server
		expectSuccess bool
		neighbor      nodeData
	}{
		{
			name: "successful retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("image data"))
			})),
			expectSuccess: true,
			neighbor: nodeData{
				Nickname:  "neighbor1",
				UUID:      "neighbor1-uuid",
				Writeable: true,
			},
		},
		{
			name: "failed retrieval",
			server: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})),
			expectSuccess: false,
			neighbor: nodeData{
				Nickname:  "neighbor1",
				UUID:      "neighbor1-uuid",
				Writeable: true,
			},
		},
		{
			name:          "myself node",
			server:        httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			expectSuccess: false,
			neighbor:      nodeData{UUID: "test-uuid"},
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

			// Create a cluster.
			_, c := makeNewClusterData([]nodeData{})
			tn := tt.neighbor
			tn.BaseURL = tt.server.URL
			c.AddNeighbor(tn)

			// Retrieve the image from the cluster.
			img, err := c.RetrieveImage(context.Background(), ri)

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

func TestClusterUpdateNeighbor(t *testing.T) {
	logger := log.NewNopLogger()
	_, c := makeNewClusterData([]nodeData{})
	neighbor := nodeData{
		Nickname:  "neighbor1",
		UUID:      "neighbor1-uuid",
		Writeable: true,
	}

	// test adding a new neighbor
	c.updateNeighbor(neighbor, logger)
	neighbors := c.GetNeighbors()
	if len(neighbors) != 1 {
		t.Fatal("Expected 1 neighbor, got", len(neighbors))
	}
	if neighbors[0].Nickname != "neighbor1" {
		t.Error("Wrong neighbor added")
	}

	// test updating an existing neighbor
	neighbor.Nickname = "neighbor-updated"
	c.updateNeighbor(neighbor, logger)
	neighbors = c.GetNeighbors()
	if len(neighbors) != 1 {
		t.Fatal("Expected 1 neighbor, got", len(neighbors))
	}
	if neighbors[0].Nickname != "neighbor-updated" {
		t.Error("Neighbor not updated")
	}

	// test skipping myself
	myself := c.Myself
	c.updateNeighbor(myself, logger)
	neighbors = c.GetNeighbors()
	if len(neighbors) != 1 {
		t.Fatal("Expected 1 neighbor, got", len(neighbors))
	}
}
