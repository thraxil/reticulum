---
title: Reticulum API
layout: layout
---

# API

## Public API

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

There is also a `/status` URL available on each node that gives a
rough, human-readable summary of the node's status and what it knows
about the cluster.

## Internal API

This section documents how nodes talk to each other. 

### Image Stash

### Image Retrieval

### Gossip
