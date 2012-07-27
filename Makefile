all: reticulum

reticulum: reticulum.go models/models.go views/views.go resize_worker/worker.go node/node.go cluster/cluster.go verifier/verifier.go config/config.go
	go build reticulum.go

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
	go fmt views/views.go
	go fmt verifier/verifier.go
	go fmt resize_worker/worker.go
	go fmt models/models.go
	go fmt cluster/cluster.go
	go fmt node/node.go
	go fmt config/config.go

install: reticulum
	cp -f reticulum /usr/local/bin/reticulum

test: reticulum
	go test ./node ./cluster

coverage: reticulum
	../go/bin/gocov test ./node | ../go/bin/gocov report
	../go/bin/gocov test ./cluster ../go/bin/gocov report

