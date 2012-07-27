package cluster

import (
	_ "fmt"
	"testing"
	"../node"
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
	if len(c.Neighbors) != 0 {
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


}


