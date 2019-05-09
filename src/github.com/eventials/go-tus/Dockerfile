FROM golang:1.9

RUN mkdir -p /go/src/github.com/eventials/go-tus

WORKDIR /go/src/github.com/eventials/go-tus

RUN go get github.com/stretchr/testify
RUN go get github.com/tus/tusd
RUN go get github.com/syndtr/goleveldb/leveldb
