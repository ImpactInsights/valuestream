FROM golang:1.12

WORKDIR /go/src/github.com/ImpactInsights/valuestream
COPY . .

RUN GO111MODULE=on go get -d -v ./...
RUN GO111MODULE=on go install -v ./...

EXPOSE 5000

RUN useradd -m vs
USER vs

CMD valuestream -addr=":"$PORT

