package models

import "time"

type Record struct {
	ID          int        `gorm:"column:id;primaryKey;autoIncrement"`
	From        string     `gorm:"column:from;type:varchar(320);not null"`
	To          string     `gorm:"column:to;type:varchar(320);not null"`
	Subject     string     `gorm:"column:subject;type:varchar(255);not null"`
	Body        string     `gorm:"column:body;type:text;not null"`
	ContentType string     `gorm:"column:content_type;type:varchar(32);not null;default:text/plain"`
	Status      int        `gorm:"column:status;not null"`
	SentAt      *time.Time `gorm:"column:sent_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (*Record) TableName() string {
	return "records"
}

func AllModels() []interface{} {
	return []interface{}{&Record{}}
}
