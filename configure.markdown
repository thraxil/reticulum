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

### UploadDirectory

### NumResizeWorkers

### Replication

### GossiperSleep

### VerifierSleep

### Writeable

### Neighbors

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
