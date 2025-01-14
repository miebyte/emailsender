# builder
FROM golang:1.23.4-alpine AS builder

WORKDIR /app
COPY ./ /app

ENV GO111MODULE=auto
ENV GOPROXY=https://goproxy.cn,direct
RUN cd /app && \
	go mod tidy && \
	go build -o ./server && \
	chmod +x server

# runner
FROM alpine:latest
ARG GO_PUZZLE_SERVICE 
ENV GO_PUZZLE_SERVICE=$GO_PUZZLE_SERVICE

WORKDIR /app
COPY --from=builder /app/server /app/server
