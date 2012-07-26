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

There is an 'install' target for make too that will plop it into
`/usr/local/bin/`:

    sudo make install

### dependencies 

The only non standard library dependencies it has is `resize` available
with 

    go get github.com/thraxil/reticulum

And `memcache`: 

    go get github.com/bradfitz/gomemcache/memcache

## Download Binaries

TODO

## Running

The way I run it on my systems is via Ubuntu's `upstart` system. I
make a file in `/etc/init/` called `reticulum.conf` with contents
like:

    description "start/stop reticulum"
    version "1.0"
    author "Anders Pearson"
    
    expect fork
    script
    exec sudo -u anders /usr/local/bin/reticulum -config=/mnt/sata1/reticulum/config.json &
    end script

And then do:

    # start reticulum
    # stop reticulum
    # restart reticulum

etc. Add it to default runlevels, whatever.

See Ubuntu's [Upstart Cookbook](http://upstart.ubuntu.com/cookbook/)
for more information on that whole system. Supervisord or SysV init or
anything similar would also work fine. Recipe contributions are welcome.

See the [configuration](configure.html) section for more info on what
to put in the config.json. 
