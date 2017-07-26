package main

type sharedChannels struct {
	ResizeQueue chan resizeRequest
}

type imageRecord struct {
	Hash      hash
	Extension string
}
