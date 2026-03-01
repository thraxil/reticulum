package main

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
)

type LogEntry struct {
	Timestamp  string      `json:"timestamp"`
	Level      string      `json:"level"`
	Message    interface{} `json:"msg"`
	Caller     string      `json:"caller"`
	Component  string      `json:"component"`
	Node        string      `json:"node"`
	RemoteAddr  string      `json:"remote_addr"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Duration    string      `json:"time"`
	Error       interface{} `json:"error"`
	Image       string      `json:"image"`
	Replication int         `json:"replication"`
	Raw         string      `json:"-"`
}

type LogCache struct {
	lines []string
	size  int
	mu    sync.Mutex
}

func NewLogCache(size int) *LogCache {
	return &LogCache{
		lines: make([]string, 0, size),
		size:  size,
	}
}

func (lc *LogCache) Write(p []byte) (n int, err error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.lines = append(lc.lines, string(p))
	if len(lc.lines) > lc.size {
		lc.lines = lc.lines[len(lc.lines)-lc.size:]
	}
	return len(p), nil
}

func (lc *LogCache) Entries() []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	// Return a copy to avoid races
	result := make([]string, len(lc.lines))
	copy(result, lc.lines)
	return result
}

func (lc *LogCache) StructuredEntries() []LogEntry {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	// Return a copy to avoid races
	entries := make([]LogEntry, 0, len(lc.lines))
	for _, line := range lc.lines {
		var e LogEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			e.Raw = line
			if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
				e.Timestamp = t.Format("2006-01-02 15:04:05")
			}
			entries = append(entries, e)
		} else {
			entries = append(entries, LogEntry{Raw: line, Message: line})
		}
	}
	return entries
}

var GlobalLogCache *LogCache

func init() {
	GlobalLogCache = NewLogCache(100)
}

func newSTDLogger() log.Logger {
	w := log.NewSyncWriter(io.MultiWriter(os.Stderr, GlobalLogCache))

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
