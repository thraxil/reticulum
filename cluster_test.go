package main

import (
	"fmt"
	"testing"
)

// need a global one to avoid calling groupcache.NewHTTPool
// multiple times
var cluster *Cluster

func makeNewClusterData(neighbors []NodeData) (NodeData, *Cluster) {
	myself := NodeData{
		Nickname:  "myself",
		UUID:      "test-uuid",
		BaseUrl:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}

	if cluster == nil {

		gcp := &GroupCacheProxy{}
		c := NewCluster(myself, gcp)
		for _, n := range neighbors {
			c.AddNeighbor(n)
		}
		cluster = c
		return myself, cluster
	} else {
		cluster.Myself = myself
		cluster.neighbors = map[string]NodeData{}
		cluster.gcpeers.Set()
		return myself, cluster
	}
}

func Test_ClusterOfOneInitialNeighbors(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
}

func Test_ClusterOfOneNeighborsInclusive(t *testing.T) {
	n := make([]NodeData, 0)
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
	n := make([]NodeData, 0)
	myself, c := makeNewClusterData(n)
	neighbors := c.NeighborsInclusive()

	_, found := c.FindNeighborByUUID("test-uuid")
	if found {
		t.Error("neighbors should be empty")
	}

	neighbors = c.WriteableNeighbors()
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

func checkForNeighbor(c *Cluster, n NodeData, t *testing.T) {
	rn, found := c.FindNeighborByUUID(n.UUID)
	if !found {
		t.Error(fmt.Sprintf("couldn't find %s by UUID", n.UUID))
	}
	if rn.Nickname != n.Nickname {
		t.Error("not the same nickname")
	}
}

func checkForNeighborAfterRemoval(c *Cluster, n NodeData, i int, t *testing.T) {
	rn, found := c.FindNeighborByUUID(n.UUID)
	if i == 2 {
		// the one that was removed
		if found {
			t.Error("found the one we removed")
		}
	} else {
		if !found {
			t.Error(fmt.Sprintf("couldn't find %s by UUID", n.UUID))
		}
		if rn.Nickname != n.Nickname {
			t.Error("not the same nickname")
		}
	}
}

func Test_AddNeighbor(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	c.AddNeighbor(NodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseUrl:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	})
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}

}

func Test_RemoveNeighbor(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := NodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseUrl:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	nd2 := NodeData{
		Nickname:  "addedneighbor3",
		UUID:      "test-uuid-3",
		BaseUrl:   "localhost:8082",
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
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := NodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseUrl:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	c.AddNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}
	nd.BaseUrl = "localhost:8082"
	c.UpdateNeighbor(nd)
	neighbors := c.GetNeighbors()
	if neighbors[0].BaseUrl != "localhost:8082" {
		t.Error("update didn't take")
	}
}

func Test_FailedNeighbor(t *testing.T) {
	n := make([]NodeData, 0)
	_, c := makeNewClusterData(n)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}
	nd := NodeData{
		Nickname:  "addedneighbor",
		UUID:      "test-uuid-2",
		BaseUrl:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	c.AddNeighbor(nd)
	if len(c.GetNeighbors()) != 1 {
		t.Error("should be only one")
	}
	nd.BaseUrl = "localhost:8082"
	c.FailedNeighbor(nd)
	neighbors := c.GetNeighbors()
	if neighbors[0].Writeable {
		t.Error("failed notification didn't take")
	}
}
