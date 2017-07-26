package main

import "github.com/golang/groupcache"

type GroupCacheProxy struct{}

func (g *GroupCacheProxy) MakeInitialPool(url string) PeerList {
	return groupcache.NewHTTPPool(url)
}

func (g *GroupCacheProxy) MakeCache(c *Cluster, size int64) CacheGetter {
	return groupcache.NewGroup(
		"ReticulumCache", size, groupcache.GetterFunc(
			func(ctx groupcache.Context, key string, dest groupcache.Sink) error {

				// get image from disk
				ri := NewImageSpecifier(key)
				img_data, err := c.RetrieveImage(ri)
				if err != nil {
					return err
				}
				dest.SetBytes([]byte(img_data))
				return nil
			}))
}
