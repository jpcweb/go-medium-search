FROM golang:1.11.5-alpine3.9

RUN apk add --no-cache git bash
RUN go get golang.org/x/net/html

WORKDIR /go/src/app

COPY . .

RUN go build -o app .

EXPOSE 8080

ENTRYPOINT ["./app"]
