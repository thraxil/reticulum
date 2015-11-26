FROM centurylink/ca-certs
COPY reticulum /
EXPOSE 2000
ENTRYPOINT ["/reticulum", "-config=/config.json"]
