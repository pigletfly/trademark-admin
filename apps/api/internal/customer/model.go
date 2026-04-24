package customer

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Customer mirrors the customers table. Soft-delete via deleted_at.
type Customer struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name           string    `gorm:"not null"`
	Industry       *string
	IsReturning    bool `gorm:"not null;default:false"`
	PriceSensitive bool `gorm:"not null;default:false"`
	ContactName    *string
	ContactPhone   *string
	ContactEmail   *string
	Notes          *string
	CreatedBy      uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName pins the GORM mapping.
func (Customer) TableName() string { return "customers" }
