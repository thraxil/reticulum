---
title: Reticulum Configuration
layout: layout
---

# Configuration

When you run reticulum, you give tell it what config file to run:

     ./reticulum -config=/path/to/config.json

The config file should be json formatted and supports the following
fields:

## Fields

### Nickname

Human-readable name for this node. Recommend you keep it short,
unique, and basic alphanumeric with no punctuation or spaces in it.

### UUID

Globally unique ID for the node. This MUST be unique for the
cluster. It is the "name" of the node as far as other nodes in the
cluster are concerned. It is also used to determine what section of
the Ring this node claims, so it should not change once it is set (if
it changes, there could be significant churn as the cluster will want
to move images around to suit its new position in the Ring). Highly
recommended you use an actual UUID/GUID. The `uuidgen` command that is
found on most Unix systems will generate a guaranteed unique UUID for
you if you don't have a preferred scheme. Highly recommended you use
it. 

It's not required, but it's recommended again to keep this short and
simple with just alphanumerics and dashes. Eg, a `uuidgen` generated
UUID would look like:
`216ce54c-3555-4769-a34b-1085bc4cf042`. Reticulum will just treat it
as a black box string, but for future-proofing, it is highly
recommended that you keep in mind that the intention is for this field
to have a format something like that.

### Port

The TCP port to listen on. 

### UploadDirectory

Directory to store uploaded images. If it does not start with `/`, it
will be relative to the directory that reticulum is started from. You
probably want to give it an absolute path though.

### NumResizeWorkers

Maximum number of simultaneous resize operations to allow. If this is
the only node on a system, you probably want it somewhere around as
many CPU cores as you have. Experiment and benchmark for best results
though. 

### Replication

Number of copies of each uploaded image it will try to maintain. 

### MinReplication

Number of replicas created on upload to consider "success". In the
future, once a node has confirmed that at least MinReplication nodes
have stored an image, it will return a success value to the client and
allow any other stashing (to fully satisfy `Replication`) to happen in
the background.

### MaxReplication

When the verifier runs, for each image, it checks to see how many
nodes have copies. Ideally, this is `Replication`. With nodes joining
and leaving, there can sometimes be more replicas in the cluster than
you wanted. `MaxReplication` sets the threshold at which the verifier
starts cleaning up excess replicas. You generally want to set this
equal to or a little bit higher than `Replication`. Overall, your
entire cluster should basically agree on `Replication`,
`MinReplication`, and `MaxReplication` values to avoid unnecessary
churn. 

### GossiperSleep

How many seconds to sleep in between gossip pings. 

### VerifierSleep

How many seconds to sleep in between the verification of each image in
the uploads directory. Increase this if reticulum is wasting too much
time and CPU verifying and rebalancing your images.

### Writeable

Will this node accept images? You can potentially run a read-only node
that will accept new images but will still serve the ones that it
has. This is appropriate if you've got a full disk or want to serve
from a read-only data store. 

### ImageMagickConvertPath

Reticulum grudgingly uses imagemagick's `convert` program to scale
progressive JPEGs since Go's included `image/jpeg` library cannot read
or write them. This is something that we hope to change in the future
once Go's standard library fills in that void. In the meantime, you
need to have imagemagick installed. Reticulum defaults to
`/usr/bin/convert` as the path to the `convert` program. If that is
not the case on your system, change the path here.

### MemcacheServers

It's a very good idea to set up your front-line node(s) to use
memcached so images that have been served up recently will be
available directly in RAM. This will make a huge difference in almost
any setup. This field just takes a list of strings of memcache
"IP:PORT" servers. See the
[gomemcache](https://github.com/bradfitz/gomemcache/) for more
info. This just gets passed directly to `memcache.New()`. 

### Neighbors

List of known neighbors to start with. Each has `Nickname`, `UUID`,
`BaseUrl`, and `Writeable` fields. `Nickname` and `Writeable` are
advisory only. `UUID` and `BaseUrl` are necessary to bootstrap the
cluster. It doesn't need to know about the entire cluster here since
the gossiper will eventually find out about everyone, but this lets
the node start participating as soon as it starts up. 

## Example Config

This is just an example from the `test` directory:

    {"Nickname": "node0",
    "UUID" : "216ce54c-3555-4769-a34b-1085bc4cf042",
    "Port" : 8080,
    "UploadDirectory" : "test/uploads0/",
    "NumResizeWorkers" : 4,
    "Replication" : 3,
    "GossiperSleep" : 10,
    "VerifierSleep" : 60,
    "Writeable" : true,
    "MemcacheServers" : ["127.0.0.1:11211"],
    "Neighbors": [
                 {"Nickname": "node1",
                 "UUID": "uuid-node-1",
                 "BaseUrl": "localhost:8081",
                 "Writeable": true
                 },
                 {"Nickname": "node2",
                 "UUID": "uuid-node-2",
                 "BaseUrl": "localhost:8082",
                 "Writeable": true
                 },
                 {"Nickname": "node3",
                 "UUID": "uuid-node-3",
                 "BaseUrl": "localhost:8083",
                 "Writeable": true
                 },
                 {"Nickname": "node4",
                 "UUID": "uuid-node-4",
                 "BaseUrl": "localhost:8084",
                 "Writeable": true
                 },
                 {"Nickname": "node5",
                 "UUID": "uuid-node-5",
                 "BaseUrl": "localhost:8085",
                 "Writeable": true
                 },
                 {"Nickname": "node6",
                 "UUID": "uuid-node-6",
                 "BaseUrl": "localhost:8086",
                 "Writeable": true
                 },
                 {"Nickname": "node7",
                 "UUID": "uuid-node-7",
                 "BaseUrl": "localhost:8087",
                 "Writeable": true
                 },
                 {"Nickname": "node8",
                 "UUID": "uuid-node-8",
                 "BaseUrl": "localhost:8088",
                 "Writeable": true
                 },
                 {"Nickname": "node9",
                 "UUID": "uuid-node-9",
                 "BaseUrl": "localhost:8089",
                 "Writeable": true
                 }
      ]
    }
