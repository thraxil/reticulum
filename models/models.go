package models

import (
	"../resize_worker"
)

type SharedChannels struct {
	ResizeQueue chan resize_worker.ResizeRequest
}
