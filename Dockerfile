FROM golang:alpine3.19 AS builder

RUN mkdir /app
WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download

COPY . /app/
RUN CGO_ENABLE=0 go build -ldflags '-extldflags "-static"' -tags timetzdata

FROM scratch

COPY --from=builder /app/immich-stacker /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/immich-stacker"]
CMD [""]
