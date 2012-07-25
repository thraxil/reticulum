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
* entropy based cropping (find the part of the image with the most entropy and crop so that ends up in the center)
* send image dimensions in HTTP headers
* node generation boilerplace command (like 'tahoe create')
* the resize library should support a fast vs good toggle to allow it
* to resize via google's graphics-go (fast) or the better looking, but significantly slower resize algo. Then the node can make executive decisions like "a 50px thumbnail does not need to look that good" or do multi-stage resize like apomixis did.
* listen for a HUP signal to re-read config 
* uuid generation tool
* read repair: when a node serves up an image, start a background job that runs a verify/rebalance on the image.


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
