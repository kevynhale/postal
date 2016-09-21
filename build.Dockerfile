FROM golang:1.7-alpine

RUN apk update && apk add git bash gcc mercurial py-pip

RUN go get github.com/Masterminds/glide && \
  go get github.com/mitchellh/gox  && \
  go get github.com/golang/lint/golint && \
  go get github.com/axw/gocov/gocov && \
  go get gopkg.in/matm/v1/gocov-html

WORKDIR /go/src/github.com/jive/postal

COPY glide.yaml glide.yaml
COPY glide.lock glide.lock
RUN glide i --cache-gopath --use-gopath

COPY . /go/src/github.com/jive/postal
