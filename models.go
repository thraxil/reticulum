package main

type SharedChannels struct {
	ResizeQueue chan ResizeRequest
}

type ImageRecord struct {
	Hash      Hash
	Extension string
}
