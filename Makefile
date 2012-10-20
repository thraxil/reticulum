all: reticulum

reticulum: reticulum.go models.go views.go worker.go node.go cluster.go verifier.go config.go
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
	go fmt reticulum.go
	go fmt views.go
	go fmt verifier.go
	go fmt worker.go
	go fmt models.go
	go fmt cluster.go
	go fmt node.go
	go fmt config.go

install: reticulum
	cp -f reticulum /usr/local/bin/reticulum

test: reticulum
	go test .

coverage: reticulum
	${GOROOT}/bin/gocov test . | ${GOROOT}/bin/gocov report


install_deps:
	go get github.com/thraxil/resize
	go get github.com/bradfitz/gomemcache/memcache
