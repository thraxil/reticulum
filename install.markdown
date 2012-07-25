---
title: Reticulum Install
layout: layout
---

# Install

## Compile from source

Assuming you have a Go compiler installed and set up (I've only tested
with the main Go compiler but I see no reason that `gccgo` wouldn't
also work). 

There's a good chance that:

    go get github.com/thraxil/reticulum

Will do it.

Otherwise clone it and do

    make

and that will create a `reticulum` binary for your platform.

The only non standard library dependencies it has is `resize` available
with 

    go get github.com/thraxil/reticulum

And `memcache`: 

    go get github.com/bradfitz/gomemcache/memcache

## Download Binaries

TODO
