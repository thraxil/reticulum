package main

import ()

type Logger interface {
	Info(m string) (err error)
	Err(m string) (err error)
	Warning(m string) (err error)
}
