package models

import (
	"github.com/thraxil/reticulum/resize_worker"
)

type SharedChannels struct {
	ResizeQueue chan resize_worker.ResizeRequest
}
