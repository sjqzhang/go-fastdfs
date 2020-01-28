# Go parameters
GO=go
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOGET=$(GO) get
BINARY_NAME=go-fastdfs
BINARY_UNIX=$(BINARY_NAME)_unix

all: test build
build:
	$(GOBUILD) -v -tags=jsoniter -o $(BINARY_NAME) ./cmd/fastdfs/fastdfs.go
test:
	# TODO, add test, $(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
run:
	$(GOBUILD) -v -tags=jsoniter -o $(BINARY_NAME) ./cmd/fastdfs/fastdfs.go
	./$(BINARY_NAME)
deps:
	#$(GOGET) github.com/luoyunpeng/...

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v
docker-build:
	docker run --rm -it -v "$(GOPATH)":/go -w /go/src/github.com/luoyunpeng/go-fastdfs golang:1.13.5-alpine go build -v -tags=jsoniter -o "$(BINARY_NAME)" ./cmd/fastdfs/fastdfs.go
