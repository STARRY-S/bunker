#!/bin/bash

# Build proxy executable binnary

cd $(dirname $0)/../

set -exuo pipefail

mkdir -p build && cd build

CGO_ENABLED=0 \
    go build -a -ldflags '-extldflags "-static"' -o bunker ..

./bunker version

echo "build: Done"
