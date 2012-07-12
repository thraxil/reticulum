all: reticulum

reticulum: reticulum.go models/models.go views/views.go resize_worker/worker.go
	go build reticulum.go

test: reticulum
	python run_cluster.py

clean:
	rm reticulum

testclean:
	rm -rf test/uploads*
