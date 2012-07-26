---
title: Reticulum Architecture
layout: layout
---

# Architecture

## Components

### Node

Each reticulum instance in a cluster is a Node. It runs as its own
process, has its own storage, listens on a port for HTTP requests,
runs its own set of resize workers, its own gossiper, and its own
verifier.

### Cluster

Each node maintains its own representation of the whole cluster from
its own perspective. This is basically just the set consisting of
itself, plus all of its known (living) neighbors. When it finds out
about a new neighbor, it integrates it into its cluster. For reading
and writing, it maintains a Ring based on the current state of the
cluster. 

On a reliable network with minimal node churn, each node should fairly
quickly settle out to about the same representation of the overall
cluster. It is important to understand that this isn't totally
necessary. Reticulum is designed to still operate efficiently even
when nodes disagree about the overall cluster. Eg, a weird network
issue could prevent two nodes in a cluster from seeing each other,
while they can both see all the other nodes in the cluster. Reticulum
should be robust about that kind of situation (though things are more
efficient when fully connected). 

### Ring

The Ring is a structure created for the distributed hash table that
Reticulum uses. Each node in the cluster generates N (currently
defaults to 16) hashes in a predictable manner (based off its
UUID). Those hashes from all the nodes in the cluster are mapped onto
a ring and each part of that ring is "owned" by the Node with the next
hash in order. When an image needs to be read or written, N hashes are
generated for that image in the same predictable manner (using the
image's SHA-1, which is also its storage/retrieval key as the
basis). Those hashes are then mapped onto the main Ring and used to
determine an ordering of Nodes to be queried (or saved to). 

If all the nodes in the cluster have the same list of nodes (and their
UUIDs), then a given image will produce the same ordered list of nodes
on every node without any additional communication between the nodes
required. On a large cluster though, there's a very good chance that
some of the nodes will disagree slightly on what neighbors are
available. The beauty of this DHT approach is that as long as they
*mostly* agree (and the gossip protocol makes this a reasonable
expectation asymptotically), they will still come up with very nearly
the same ordered lists of nodes and storage and retrieval will be
minimally affected by nodes leaving and joining the cluster. 

It is worth noting that each node actually maintains two rings, one
for reading and one for writing. They are essentially the same except
the Write Ring only contains nodes that are identified as writeable
(since there can be read-only nodes). If all nodes are writeable, they
will be identical.

### Gossiper

The gossiper periodically pings neighbors and checks their status. It
also keeps an eye out for new nodes that a neighbor knows about that
our node doesn't yet and adds them to the cluster.

### Verifier

The verifier runs as a background goroutine that periodically walks
the storage directory looking at each image that the node has
stored. It calculates the hash of the image and compares it to the
storage location (which is based on the original hash) to make sure
the image hasn't become corrupted. If it has, it tries to repair it by
requesting a correct copy from other nodes that have replicas. 

It then checks the cluster to see which other nodes have replicas of
the image. If any of the nodes that are at the front of the Ring for
that image don't have replicas, it sends the image to them (so future
retrievals will succeed with the minimum number of queries). If the
current node is not near the front of that image's list and it has
verified that other nodes have a sufficient number of replicas, it
deletes the local copy to free up space.

### Resize Worker

The resize worker does the actual image scaling. Each node maintains a
pool of them (configurable). All resize requests for that node are
processed by that pool in FIFO order. 

### Views

These are just the basic HTTP handlers.
