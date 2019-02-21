FROM registry.cn-hangzhou.aliyuncs.com/prince/alpine-golang:1.11.5 as builder
MAINTAINER prince <8923052@qq.com>
ARG VERSION=1.1.7
RUN set -xe; \
	apk update; \
	apk add --no-cache --virtual .build-deps \
	git; \
	cd /go/src/; \
	git clone https://github.com/sjqzhang/go-fastdfs.git; \
	cd go-fastdfs; \
	git checkout v${VERSION}; \
	go get; \
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fileserver; \
	ls -lh .;
FROM registry.cn-hangzhou.aliyuncs.com/prince/alpine-bash

COPY --from=builder /go/src/go-fastdfs/fileserver /

ENV INSTALL_DIR /usr/local/go-fastdfs

ENV PATH $PATH:$INSTALL_DIR/

ENV GO_FASTDFS_DIR $INSTALL_DIR/data

RUN set -xe; \
	mkdir -p $GO_FASTDFS_DIR; \
	mkdir -p $GO_FASTDFS_DIR/conf; \
	mkdir -p $GO_FASTDFS_DIR/data; \
	mkdir -p $GO_FASTDFS_DIR/files; \
	mkdir -p $GO_FASTDFS_DIR/log; \
	mkdir -p $INSTALL_DIR; \
	mv /fileserver $INSTALL_DIR/; \
	chmod +x $INSTALL_DIR/fileserver;

WORKDIR $INSTALL_DIR

VOLUME $GO_FASTDFS_DIR

CMD ["fileserver" , "${OPTS}"]