package main

import "log"

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
}

func (s STDLogger) Info(m string) error {
	log.Println("INFO", m)
	return s.InfoResponse
}
func (s STDLogger) Err(m string) error {
	log.Println("ERR", m)
	return s.ErrResponse
}
func (s STDLogger) Warning(m string) error {
	log.Println("WARN", m)
	return s.WarningResponse
}
