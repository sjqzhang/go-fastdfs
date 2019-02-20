FROM golang:1.7-alpine AS builder

# Copy in the git repo from the build context
COPY . /go/src/github.com/tus/tusd/

# Create app directory
WORKDIR /go/src/github.com/tus/tusd

RUN apk add --no-cache \
        git \
    && go get -d -v ./... \
    && version="$(git tag -l --points-at HEAD)" \
    && commit=$(git log --format="%H" -n 1) \
    && GOOS=linux GOARCH=amd64 go build \
        -ldflags="-X github.com/tus/tusd/cmd/tusd/cli.VersionName=${version} -X github.com/tus/tusd/cmd/tusd/cli.GitCommit=${commit} -X 'github.com/tus/tusd/cmd/tusd/cli.BuildDate=$(date --utc)'" \
        -o "/go/bin/tusd" ./cmd/tusd/main.go \
    && rm -r /go/src/* \
    && apk del git

# start a new stage that copies in the binary built in the previous stage
FROM alpine:3.8

COPY --from=builder /go/bin/tusd /usr/local/bin/tusd

RUN apk add --no-cache ca-certificates \
    && addgroup -g 1000 tusd \
    && adduser -u 1000 -G tusd -s /bin/sh -D tusd \
    && mkdir -p /srv/tusd-hooks \
    && mkdir -p /srv/tusd-data \
    && chown tusd:tusd /srv/tusd-data

WORKDIR /srv/tusd-data
EXPOSE 1080
ENTRYPOINT ["tusd"]
CMD ["--hooks-dir","/srv/tusd-hooks"]
