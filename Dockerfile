FROM centurylink/ca-certs
COPY reticulum /
ENTRYPOINT ["/reticulum", "-config=/config.json"]
