all: reticulum

reticulum: *.go
	go build .

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


install_deps:
	go get -u github.com/thraxil/resize
	go get -u github.com/golang/groupcache

# local dev helpers
stopall:
	sudo stop reticulum-sata1
	sudo stop reticulum-sata2
	sudo stop reticulum-sata4
	sudo stop reticulum-sata7
	sudo stop reticulum-sata8
	sudo stop reticulum-sata9
	sudo stop reticulum-sata10
	sudo stop reticulum-sata11
	sudo stop reticulum-sata12

startall:
	sudo start reticulum-sata1
	sudo start reticulum-sata2
	sudo start reticulum-sata4
	sudo start reticulum-sata7
	sudo start reticulum-sata8
	sudo start reticulum-sata9
	sudo start reticulum-sata10
	sudo start reticulum-sata11
	sudo start reticulum-sata12
