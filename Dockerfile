FROM golang:1.8
RUN apt-get update && apt-get install -y \
    imagemagick \
		&& rm -rf /var/lib/apt/lists/*

# mount your data directory here
RUN mkdir -p /var/data/reticulum/

# build it
WORKDIR /go/src/app
COPY . .
RUN go-wrapper install

# http
EXPOSE 8080

# groupcache
EXPOSE 10080

CMD ["go-wrapper", "run"]
