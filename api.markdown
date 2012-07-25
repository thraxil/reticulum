---
title: Reticulum API
layout: layout
---

# API

## Public API

### Upload

To add an image to the cluster make a `POST` request to `/`. The only
required parameter is `image` and it should be
`multipart/form-data`. A sample `curl`:

    curl -X POST -F image=@someimage.jpg http://reticulum.example.com/

Reticulum servers can also be configured to require a "key" on
upload (so the upload API isn't wide open). If you set that up, you'll
need to send that along as well, obviously:

    curl -X POST -F key=somesecretkey -F image=@someimage.jpg http://reticulum.example.com/

The response from either of those on success will be JSON with the
hash of the image (which you will want to use in the future to
retrieve the image from the cluster), the path to the full-size image
(redundant since you get the hash, but handy for manual testing), and
a list of the nodes that the image was stored to (again, mostly for
manual testing).

### Get an Image

You need the hash of image (which Reticulum gives you when you
upload), the extension, and the specification of the size you
want. Then just make a `GET` request like: 

    GET http://reticulum.example.com/image/<hash>/<size>/image.<extension>

The `image.<extension>` part can also be changed. Any string can be
used as the base of the filename. Reticulum doesn't keep track of what
the original filename was, but if your client does, that might be a
good place to put it. The only real advantage to doing that is if
there are reticulum served images in a page and you want to let users
right-click and download them, you can make it so they don't end up
with `image.jpg`, `image (1).jpg`, `image (2).jpg`, etc.

#### Sizes

Sizes are specified in a simple way. There are three different
parameters: `width`, `height`, and `square`. Width and height can be
specified individually or together. If `square` is specified, any
`width` or `height` specs are ignored.

Each parameter has a one-letter code (`w`, `h`, and `s`, respectively)
and an integer value. They are put together with the with number
first, then the code. Examples should make this very clear:

    100w = scale the image so it's at most 100px wide
    200h = scale the image so it's at most 200px tall
    600s = crop and scale the image to a square that's at most 600px
    100w100h = scale the image so it's at most 100px on a side (no cropping)

If specifying both `w` and `h`, always specify `w` first. This isn't
enforced currently, but may be in the future (via 301 redirects).

### Cluster Status

There is also a `/status` URL available on each node that gives a
rough, human-readable summary of the node's status and what it knows
about the cluster.

## Internal API

This section documents how nodes talk to each other. 

### Image Stash

What an image is uploaded (or the verifier rebalances), replicas are
handed off to other nodes via a stash. This is very similar to the
regular client upload. Just a POST request to `/stash/` with the
image. Response is just success/failure.

### Image Retrieval

When a node has a request for an image and doesn't have it locally, it
will ask one or more other nodes for the image (using the ring to make
an educated guess about which node(s) are likeliest to have a
copy). It does this by making a `GET` request to
`/retrieve/<hash>/<size>/<ext>/`. Response will be either a 404 or the
desired image. 

### Gossip

Each node periodically goes through it's list of known neighbors and
pings each one. This ping is a two-way street. When node A pings node
B, it sends its own info as well as the list of neighbors that it
knows about. Node B will update its neighbor list based on that
(updating the LastSeen timestamp for Node A and adding any neighbors
from A's list that were not already in its own list). Node B responds
to the ping with its info and its list of known neighbors and A
handles that information similarly, updating LastSeen for B, and
merging in the neighbor list from B. 

The mechanism is just a `POST` request from A to B's `/announce/`
URL. The `POST` parameters are the basic info about node A (UUID,
BaseUrl, Writeable, etc) and the response is JSON with B's info and
neighbor list. 
