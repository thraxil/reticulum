package cluster

import (
	"fmt"
	"testing"
	"github.com/thraxil/reticulum/node"
)

func Test_ClusterOfOne(t *testing.T) {
	myself := node.NodeData{
		Nickname:  "myself",
		UUID:      "test-uuid",
		BaseUrl:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}

	c := NewCluster(myself)
	if len(c.GetNeighbors()) != 0 {
		t.Error("should not have any neighbors yet")
	}

	neighbors := c.NeighborsInclusive()
	if len(neighbors) != 1 {
		t.Error("too many neighbors for empty cluster")
	}
	if neighbors[0].Nickname != myself.Nickname {
		t.Error("single node is not myself")
	}
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

func Test_SmallCluster(t *testing.T) {
	myself := node.NodeData{
		Nickname:  "myself",
		UUID:      "test-uuid",
		BaseUrl:   "localhost:8080",
		Location:  "test",
		Writeable: true,
	}
	n1 := node.NodeData{
		Nickname:  "neighbor-1",
		UUID:      "neighbor-1-uuid",
		BaseUrl:   "localhost:8081",
		Location:  "test",
		Writeable: true,
	}
	n2 := node.NodeData{
		Nickname:  "neighbor-2",
		UUID:      "neighbor-2-uuid",
		BaseUrl:   "localhost:8082",
		Location:  "test",
		Writeable: true,
	}
	n3 := node.NodeData{
		Nickname:  "neighbor-3",
		UUID:      "neighbor-3-uuid",
		BaseUrl:   "localhost:8083",
		Location:  "test",
		Writeable: true,
	}
	n4 := node.NodeData{
		Nickname:  "neighbor-4",
		UUID:      "neighbor-4-uuid",
		BaseUrl:   "localhost:8084",
		Location:  "test",
		Writeable: true,
	}

	c := NewCluster(myself)
	c.AddNeighbor(n1)
	c.AddNeighbor(n2)
	c.AddNeighbor(n3)
	c.AddNeighbor(n4)

	if len(c.GetNeighbors()) != 4 {
		t.Error(fmt.Sprintf("%d",len(c.GetNeighbors())))
		t.Error(fmt.Sprintf("%v",c.GetNeighbors()))
//		t.Error("wrong number of neighbors")
	}
}
