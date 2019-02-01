#!/usr/bin/env bash
set -e

cd ../
GIT_SHA=`git rev-parse --short HEAD || echo "HEAD"`

export GOARCH="amd64"
export GOOS="linux"

CGO_ENABLED=0 go build -o disk-snapshot
mv disk-snapshot ./build/

if [ "$1" == "" ]; then
  cd ./build
  docker build -t=disk-snapshot:snapshot ./
fi
