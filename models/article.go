package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Article struct {
	gorm.Model
	UUID          string       `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	AuthorID      uint         `json:"author_id"`
	AuthorName    string       `json:"author_name"`
	Title         string       `json:"title"`
	Slug          string       `gorm:"uniqueIndex" json:"slug"`
	Content       string       `gorm:"type:text" json:"content"`
	Excerpt       string       `json:"excerpt"`
	FeaturedImage string       `json:"featured_image"`
	Category      string       `json:"category"`
	Published     bool         `json:"published"`
	PublishedAt   *time.Time   `json:"published_at"`
	Tags          []ArticleTag `gorm:"foreignKey:ArticleID" json:"tags"`
	ReadTime      string       `json:"read_time"`
}

func (a *Article) BeforeCreate(tx *gorm.DB) (err error) {
	a.UUID = uuid.New().String()
	return
}
