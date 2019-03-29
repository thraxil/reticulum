#!/bin/bash

set -e

export GOPATH=/tmp
make install_deps
make test
