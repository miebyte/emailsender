# builder
FROM golang:1.23.4-alpine AS builder

WORKDIR /app
COPY ./ /app

ENV GO111MODULE=auto
ENV GOPROXY="http://10.15.25.11:8081"

RUN cd /app && \
	go mod tidy && \
	go build -o ./server && \
	chmod +x server

# runner
FROM alpine:3.19

WORKDIR /app
COPY --from=builder /app/server /app/server
