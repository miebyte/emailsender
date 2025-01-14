// File:		service.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package email

import "context"

type Service interface {
	LoopAsyncTask(ctx context.Context, callbackFn func(int, error)) error
	SendMsg(ctx context.Context, subject, from, target string, msg []byte) error
	AsyncSendMsg(ctx context.Context, id int, subject, from, target string, msg []byte) error
}
