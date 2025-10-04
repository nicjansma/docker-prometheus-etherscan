FROM golang:alpine

LABEL maintainer="Nic Jansma <nic@nicj.net>"

WORKDIR /go

COPY . .

RUN go build -v -o bin/app src/app.go

EXPOSE 9205

CMD ["./bin/app"]
