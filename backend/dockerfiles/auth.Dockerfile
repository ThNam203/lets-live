FROM golang:1.23

RUN apt-get update && apt-get install -y ffmpeg curl

WORKDIR /usr/src/app

COPY ../go.mod ../go.sum ./
RUN go mod download && go mod verify

COPY ../pkg ./pkg
COPY ../auth ./auth

RUN go build -v -o /usr/local/bin/app ./auth/cmd/

# the set-udp-buffer-size.sh doesn't work smh
# TODO: somehow make it works please
# CMD ["/bin/bash", "-c", "./set-udp-buffer-size.sh && /usr/local/bin/app"]
CMD ["app"]
