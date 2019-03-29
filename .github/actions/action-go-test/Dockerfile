FROM golang:latest

LABEL "name"="Go test"

LABEL "com.github.actions.name"="Go test"
LABEL "com.github.actions.description"="Run go test on code"
LABEL "com.github.actions.icon"="package"
LABEL "com.github.actions.color"="#E0EBF5"

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
