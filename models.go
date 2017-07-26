package main

type SharedChannels struct {
	ResizeQueue chan resizeRequest
}

type ImageRecord struct {
	Hash      Hash
	Extension string
}
