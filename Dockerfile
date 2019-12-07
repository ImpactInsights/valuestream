FROM golang:1.13 as builder

WORKDIR /go/src/github.com/ImpactInsights/valuestream
COPY . .

RUN GO111MODULE=on go get -d -v ./...
RUN GO111MODULE=on go install -v ./...

FROM ubuntu

COPY --from=builder /go/bin/valuestream /usr/local/bin

RUN useradd -m vs
USER vs

CMD valuestream

