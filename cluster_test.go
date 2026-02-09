package main

import (
	"testing"
)

var gCluster *cluster

func makeNewClusterData(neighbors []nodeData) (nodeData, *cluster) {
	myself := nodeData{
		Nickname:  "myself",
		UUID:      "test-uuid",
		BaseURL:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}

	if gCluster == nil {

		c := newCluster(myself)
		for _, n := range neighbors {
			c.AddNeighbor(n)
		}
		gCluster = c
		return myself, gCluster
	}
	gCluster.Myself = myself
	gCluster.neighbors = map[string]nodeData{}
	return myself, gCluster

}

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
