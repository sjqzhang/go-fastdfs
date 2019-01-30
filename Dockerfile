FROM registry.cn-hangzhou.aliyuncs.com/prince/alpine-golang:1.11.5 as builder
MAINTAINER prince <8923052@qq.com>
ARG VERSION=1.0.9
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

RUN mkdir -p $INSTALL_DIR; \
	mkdir -p $INSTALL_DIR/conf; \
	mkdir -p $INSTALL_DIR/data; \
	mkdir -p $INSTALL_DIR/files; \
	mkdir -p $INSTALL_DIR/log; \
	mv /fileserver $INSTALL_DIR/; \
	chmod +x $INSTALL_DIR/fileserver;

ENV PATH $PATH:$INSTALL_DIR/

WORKDIR $INSTALL_DIR

CMD fileserver ${OPTS}