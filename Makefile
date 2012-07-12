all: reticulum

reticulum:
	go build reticulum.go

test: reticulum
	./reticulum

clean:
	rm reticulum
