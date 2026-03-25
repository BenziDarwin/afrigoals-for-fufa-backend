package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ArticleTag struct {
	gorm.Model
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ArticleID uint   `json:"article_id"`
	Tag       string `json:"tag"`
}

func (t *ArticleTag) BeforeCreate(tx *gorm.DB) (err error) {
	t.UUID = uuid.New().String()
	return
}
