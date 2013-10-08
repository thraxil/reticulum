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
		c := NewCluster(myself)
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

// func Test_SmallCluster(t *testing.T) {
// 	var neighbors = []NodeData{
// 		NodeData{
// 			Nickname:  "neighbor-1",
// 			UUID:      "neighbor-1-uuid",
// 			BaseUrl:   "localhost:8081",
// 			Location:  "test",
// 			Writeable: true,
// 		},
// 		NodeData{
// 			Nickname:  "neighbor-2",
// 			UUID:      "neighbor-2-uuid",
// 			BaseUrl:   "localhost:8082",
// 			Location:  "test",
// 			Writeable: true,
// 		},
// 		NodeData{
// 			Nickname:  "neighbor-3",
// 			UUID:      "neighbor-3-uuid",
// 			BaseUrl:   "localhost:8083",
// 			Location:  "test",
// 			Writeable: true,
// 		},
// 		NodeData{
// 			Nickname:  "neighbor-4",
// 			UUID:      "neighbor-4-uuid",
// 			BaseUrl:   "localhost:8084",
// 			Location:  "test",
// 			Writeable: true,
// 		},
// 	}

// 	_, c := makeNewClusterData(neighbors)

// 	if len(c.GetNeighbors()) != 4 {
// 		t.Error(fmt.Sprintf("wrong number of neighbors: %d", len(c.GetNeighbors())))
// 	}
// 	if len(c.NeighborsInclusive()) != 5 {
// 		t.Error(fmt.Sprintf("wrong number of inclusive neighbors: %d",
// 			len(c.NeighborsInclusive())))
// 	}

// 	for _, n := range neighbors {
// 		checkForNeighbor(c, n, t)
// 	}

// 	c.RemoveNeighbor(neighbors[2])
// 	if len(c.GetNeighbors()) != 3 {
// 		t.Error(fmt.Sprintf("wrong number of neighbors: %d", len(c.GetNeighbors())))
// 	}
// 	if len(c.NeighborsInclusive()) != 4 {
// 		t.Error(fmt.Sprintf("wrong number of inclusive neighbors: %d",
// 			len(c.NeighborsInclusive())))
// 	}

// 	for i, n := range neighbors {
// 		checkForNeighborAfterRemoval(c, n, i, t)
// 	}
// 	// remove the last one, just to check for off-by-ones
// 	c.RemoveNeighbor(neighbors[3])
// 	// same for the first
// 	c.RemoveNeighbor(neighbors[0])
// }
