# builder
FROM registry.hoven.top/library/golang:1.24.4 AS builder

WORKDIR /app
COPY ./ /app

ENV GOPROXY="https://goproxy.hoven.top,direct"

RUN cd /app && \
	go mod tidy && \
	go build -o ./server && \
	chmod +x server

# runner
FROM alpine:3.19

WORKDIR /app
COPY --from=builder /app/server /app/server
