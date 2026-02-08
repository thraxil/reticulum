package main

import (
	"os"

	"github.com/go-kit/log"
)

func newSTDLogger() log.Logger {
	w := log.NewSyncWriter(os.Stderr)

	logger := log.NewJSONLogger(w)
	logger = log.With(logger, "timestamp", log.DefaultTimestampUTC,
		"caller", log.DefaultCaller)
	return logger
}

// for testing
func newDummyLogger() log.Logger {
	devNull, _ := os.Open("/dev/null")
	w := log.NewSyncWriter(devNull)
	logger := log.NewJSONLogger(w)
	logger = log.With(logger, "timestamp", log.DefaultTimestampUTC,
		"caller", log.DefaultCaller)
	return logger
}
