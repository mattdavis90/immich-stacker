FROM golang:alpine3.19 AS builder

RUN mkdir /app
WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download

COPY . /app/
RUN CGO_ENABLE=0 go build -ldflags '-extldflags "-static"' -tags timetzdata

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/mattdavis90/immich-stacker
LABEL org.opencontainers.image.description="A small application to help you stack images in Immich"
LABEL org.opencontainers.image.licenses=MIT

COPY --from=builder /app/immich-stacker /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/immich-stacker"]
CMD [""]
