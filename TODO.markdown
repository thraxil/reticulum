---
title: Reticulum TODO
layout: layout
---

# TODO

Eventually, I'll move all of these (except maybe the "R&D" section) to
github issues.

### single node features

* include hash-type in exposed hashes. eg: "sha1-0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"
* secure url "drm" options
* enable multicore support
* version API
* configurable jitter range for gossiper
* configurable path to convert (until image/jpeg supports progressive jpgs)
* entropy based cropping (find the part of the image with the most entropy and crop so that ends up in the center)
* pull an image from a URL as an alternative to uploading
* handle etags + if-modified-since headers
* send image dimensions in HTTP headers
* detect noop resizes and 301 redirect to an existing one (ie, if they ask for a scaled image that's larger than the full-size)
* background goroutines for stashing replicas
* safe removal of node from cluster. It would mark itself as read-only, then walk its store of images and make sure they are each at the desired replication rate on other nodes if possible.
* node generation boilerplace command (like 'tahoe create')
* delete image functionality. This must be advisory only since it's always possible that an uploaded image has a replica on a node that is currently unreachable. Permanent delete would require maintaining (and sharing) a blacklist.
* the resize library should support a fast vs good toggle to allow it
* to resize via google's graphics-go (fast) or the better looking, but significantly slower resize algo. Then the node can make executive decisions like "a 50px thumbnail does not need to look that good" or do multi-stage resize like apomixis did.
* listen for a HUP signal to re-read config 
* uuid generation tool
* read repair: when a node serves up an image, start a background job that runs a verify/rebalance on the image.
* memcached caching of scaled images (for front-line nodes)
* recover() panics on requests (ie, don't let a 500 error crash the server or a worker)

### cluster features

* shared secret for security
* location aware replication. Ie, you've got multiple datacenters where nodes run and you want to make sure that uploaded images always get stored to each datacenter in addition to the basic replication level. This provides basic disaster recovery support. 
* bloom filters on 404s. Currently, when a node gets a request for an image it doesn't have, it will go through all its neighbors looking for a copy. In the case of a 'true' 404 where the image isn't in the cluster at all, that's expensive. A bloom filter tracking all images that the cluster has seen could let us cut our losses quicker and give the user a 404 without having to poll every single node.
* full gossip: get neighbor list from neighbors and merge
* expunge a non-responding neighbor from the list after some period (or just apply a circuit breaker pattern and ping them less and less frequently)
* weight nodes (is this a good idea?)
* timeouts EVERYWHERE. if a node doesn't respond within a set time, consider it dead.

### R&D

These are all things that I'd like to look into as possible
improvements. Many of them potentially are not improvements. Just
interesting things I want to look into when I have time. I'd
appreciate hearing thoughts from knowledgable folks.

* experiment with just using Riak for image storing via its luwak file storage. The problem with this is probably that resizing again requires downloading via HTTP, resizing, then re-uploading.
* is an external imagemagick process faster or does it use less memory?
* tiling large images?
* grayscale conversion? rounded corners? how complex image adjustment should be included? should there be a plugin API so people can write custom tweaks?
* format conversion? eg, let users upload a .tif and serve it as a .jpg?
* progressive jpeg stuff for the *really* big images. ie, it is my understanding that large medical images, etc. can be encoded with smaller, scaled versions embedded in the file so it can be read without reading the entire huge file into memory.
* LRU disk cache of all images that pass through. Basically try to obviate the need for an nginx caching proxy in front of the cluster. One node could be configured with that cache and be the "front" node. 
* should the resize workers do the actual resize work in another goroutine that is spawned for just the one job. I'm thinking that might let reticulum take care of the automatic garbage collection when a goroutine finishes. Resizing a large image can consume a lot of memory and that would more or less guarantee against leaks.
* optimistic vs strict upload mode. Ie, should it wait for confirmation from N nodes that the image has been stored before responding to the client, or are we confident that once the initial node has the file, it will probably be fine. Maybe allow a threshold in between. So we could aim for 7 replicas, but respond "ok" once we know 3 have stored it.
* SPDY support
* investigate techniques for busting through NATs
* detect duplicate UUIDs in the cluster and alert a sysadmin
* when an animated gif is scaled down, it does not keep it animated. can this be fixed?
