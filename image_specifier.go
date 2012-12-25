package main

// combination of field that uniquely specify an image
type ImageSpecifier struct {
	Hash      *Hash
	Size      string
	Extension string
}

func (i ImageSpecifier) MemcacheKey() string {
	return i.Hash.String() + "/" + i.Size + "/image" + i.Extension
}
