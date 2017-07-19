package main

import (
	"os"

	"github.com/go-kit/kit/log"
)

type Logger interface {
	Info(m string) (err error)
	Err(m string) (err error)
	Warning(m string) (err error)
}

// a dummy logger for testing purposes

type DummyLogger struct {
	InfoResponse    error
	ErrResponse     error
	WarningResponse error
}

func (d DummyLogger) Info(m string) error    { return d.InfoResponse }
func (d DummyLogger) Err(m string) error     { return d.ErrResponse }
func (d DummyLogger) Warning(m string) error { return d.WarningResponse }

type STDLogger struct {
	InfoResponse    error
	ErrResponse     error
	WarningResponse error
	writer          log.Logger
}

func NewSTDLogger() *STDLogger {
	s := STDLogger{}
	w := log.NewSyncWriter(os.Stderr)
	s.writer = log.NewJSONLogger(w)
	return &s
}

func (s STDLogger) Info(m string) error {
	s.writer.Log("level", "INFO", "msg", m)
	return s.InfoResponse
}
func (s STDLogger) Err(m string) error {
	s.writer.Log("level", "ERR", "msg", m)
	return s.ErrResponse
}
func (s STDLogger) Warning(m string) error {
	s.writer.Log("level", "WARN", "msg", m)
	return s.WarningResponse
}
