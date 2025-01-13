// File:		generate.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

//go:generate protoc -I ../../internal/proto --go_out=paths=source_relative:../../api/grpc/emailsendpb  --go-grpc_out=paths=source_relative:../../api/grpc/emailsendpb emailsend.proto
func main() {

}
