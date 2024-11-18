FROM golang:1.23

WORKDIR /usr/src/app

COPY ../go.mod ../go.sum ./
RUN go mod download && go mod verify

COPY ../pkg ./pkg
COPY ../auth ./auth

RUN go build -v -o /usr/local/bin/app ./auth/cmd/

CMD ["app"]
