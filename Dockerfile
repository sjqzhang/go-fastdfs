FROM registry.cn-hangzhou.aliyuncs.com/prince/alpine-golang:1.16 as builder
MAINTAINER s_jqzhang <s_jqzhang@163.com>
ARG VERSION=1.1.7
RUN set -xe; \
	apk update; \
	apk add --no-cache --virtual .build-deps \
	git; \
	mkdir -p /root/repo ; cd /root/repo ; \
	git clone https://github.com/sjqzhang/go-fastdfs ; \
	cd go-fastdfs; mv vendor src ; mv src .. ; cd .. ; mv go-fastdfs src/github.com/sjqzhang/ ; export GOPATH=/root/repo ; cd  src/github.com/sjqzhang/go-fastdfs ; \
	 export GO111MODULE="off"; \
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fileserver; \
	ls -lh .;
FROM registry.cn-hangzhou.aliyuncs.com/prince/alpine-bash

COPY --from=builder /root/repo/src/github.com/sjqzhang/go-fastdfs/fileserver /

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

CMD ["fileserver", "server" , "${OPTS}"]
