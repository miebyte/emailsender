// File:		record.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package model

import (
	"time"

	"github.com/miebyte/emailsender/domain/record"
	"gorm.io/gorm"
)

type Record struct {
	ID        int               `gorm:"primaryKey;autoIncrement;comment:用户主键ID"`
	From      string            `gorm:"type:varchar(100);not null;comment:邮件发送来源"`
	To        string            `gorm:"type:varchar(100);not null;comment:邮件发送目标"`
	Subject   string            `gorm:"type:varchar(50);not null;comment:邮件发送主题"`
	Message   string            `gorm:"type:text;not null;comment:邮件正文"`
	Status    record.SendStatus `gorm:"not null;comment:发送状态"`
	SentAt    *time.Time        `gorm:"comment:发送时间"`
	CreatedAt time.Time         `gorm:"comment:创建时间"`
	UpdatedAt time.Time         `gorm:"comment:更新时间"`
	DeletedAt gorm.DeletedAt    `gorm:"index;comment:软删除时间"`
}

func (r *Record) TableName() string {
	return "records"
}

func (r *Record) FromEntity(er *record.EmailSendRecord) {
	if er == nil {
		return
	}

	r.ID = er.ID
	r.From = er.From
	r.To = er.To
	r.Subject = er.Subject
	r.Message = er.Message
	r.Status = er.Status
	r.SentAt = &er.SendAt
}

func (r *Record) ToEntity() *record.EmailSendRecord {
	return &record.EmailSendRecord{
		ID:      r.ID,
		From:    r.From,
		To:      r.To,
		Subject: r.Subject,
		Message: r.Message,
		Status:  r.Status,
		SendAt:  *r.SentAt,
	}
}
