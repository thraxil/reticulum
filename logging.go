package main

import ()

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
func (d DummyLogger) Warning(m string) error { return d.ErrResponse }
