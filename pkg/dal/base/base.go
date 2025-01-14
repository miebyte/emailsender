// File:		go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package base

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

const (
	transactionDbKey = "ctx:transaction:db:key"
)

type BaseDal struct {
	*Query
}

func NewBaseDal(db *gorm.DB) *BaseDal {
	return &BaseDal{
		Query: Use(db),
	}
}

func (b *BaseDal) db(ctx context.Context) *Query {
	tsDbTmpl := ctx.Value(transactionDbKey)
	if tsDbTmpl == nil {
		return b.Query
	}

	tsDb, ok := tsDbTmpl.(*Query)
	if !ok {
		return b.Query
	}

	return tsDb
}

func (b *BaseDal) Db(ctx context.Context) *queryCtx {
	return b.db(ctx).WithContext(ctx)
}

func (b *BaseDal) SetTransaction(ctx context.Context, db *Query) context.Context {
	return context.WithValue(ctx, transactionDbKey, db)
}

func (b *BaseDal) Transaction(ctx context.Context, fc func(context.Context) error, opts ...*sql.TxOptions) error {
	db := b.db(ctx)
	return db.Transaction(func(tx *Query) error {
		return fc(b.SetTransaction(ctx, tx))
	}, opts...)
}
