#
#   Author: Rohith (gambol99@gmail.com)
#   Date: 2015-03-24 10:50:54 +0000 (Tue, 24 Mar 2015)
#
#  vim:ts=2:sw=2:et
#

NAME=bridgeio
AUTHOR=gambol99
VERSION=$(shell awk '/const Version/ { print $$4 }' version.go | sed 's/"//g')

.PHONY: build docker clean test

build:
	(mkdir -p ./bin && cd cmd/bridge && go build -o ../../bin/bridge)
	(mkdir -p ./bin && cd cmd/slow_client && go build -o ../../bin/slow_client)

docker: build
	docker build -t ${AUTHOR}/${NAME} .

clean:
	rm -f bin/bridge

test:
	go test -v ./...

