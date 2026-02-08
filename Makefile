ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
all: reticulum

reticulum: *.go
	CGO_ENABLED=0 go build .

cluster: reticulum
	python run_cluster.py

clean:
	rm -f reticulum

clusterclean:
	rm -rf test/uploads*

run: reticulum
	./reticulum -config=test/config0.json

fmt:
	go fmt *.go

install: reticulum
	cp -f reticulum /usr/local/bin/reticulum

test: reticulum
	go test .

coverage: reticulum
	go test . -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html




# local dev helpers
stopall:
	sudo systemctl stop reticulum-sata1
	sudo systemctl stop reticulum-sata2
	sudo systemctl stop reticulum-sata3
	sudo systemctl stop reticulum-sata4
	sudo systemctl stop reticulum-sata5
	sudo systemctl stop reticulum-sata6
	sudo systemctl stop reticulum-sata7
	sudo systemctl stop reticulum-sata8
	sudo systemctl stop reticulum-sata9
	sudo systemctl stop reticulum-sata10
	sudo systemctl stop reticulum-sata11
	sudo systemctl stop reticulum-sata12

startall:
	sudo systemctl start reticulum-sata1
	sudo systemctl start reticulum-sata2
	sudo systemctl start reticulum-sata3
	sudo systemctl start reticulum-sata4
	sudo systemctl start reticulum-sata5
	sudo systemctl start reticulum-sata6
	sudo systemctl start reticulum-sata7
	sudo systemctl start reticulum-sata8
	sudo systemctl start reticulum-sata9
	sudo systemctl start reticulum-sata10
	sudo systemctl start reticulum-sata11
	sudo systemctl start reticulum-sata12

build:
	docker build -t thraxil/reticulum .

push: build
	docker push thraxil/reticulum
