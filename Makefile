all: reticulum

reticulum: reticulum.go models.go views.go worker.go node.go cluster.go verifier.go config.go image_specifier.go
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
	${GOROOT}/bin/gocov test . | ${GOROOT}/bin/gocov report


install_deps:
	go get -u github.com/thraxil/resize
	go get -u github.com/bradfitz/gomemcache/memcache
	go get -u github.com/golang/groupcache
