package main

import "github.com/golang/groupcache"

type groupCacheProxy struct{}

func (g *groupCacheProxy) MakeInitialPool(url string) peerList {
	return groupcache.NewHTTPPool(url)
}

func (g *groupCacheProxy) MakeCache(c *cluster, size int64) cacheGetter {
	return groupcache.NewGroup(
		"ReticulumCache", size, groupcache.GetterFunc(
			func(ctx groupcache.Context, key string, dest groupcache.Sink) error {

				// get image from disk
				ri := newImageSpecifier(key)
				imgData, err := c.RetrieveImage(ri)
				if err != nil {
					return err
				}
				dest.SetBytes([]byte(imgData))
				return nil
			}))
}
