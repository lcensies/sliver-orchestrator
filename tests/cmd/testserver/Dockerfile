FROM golang:1.23-bullseye AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /testserver ./tests/cmd/testserver/

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends libsqlite3-0 ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY --from=build /testserver /testserver
RUN mkdir -p /data /atomics
EXPOSE 18765
ENTRYPOINT ["/testserver", "-listen=:18765", "-db=/data/test.db", "-atomics=/atomics"]
