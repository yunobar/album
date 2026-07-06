package entity

import "time"

type BaseEntity struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (be BaseEntity) IsZero() bool {
	return be.ID == 0
}
