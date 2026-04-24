package auth

import (
	"time"

	"github.com/google/uuid"
)

// Role is a platform role (salesperson, reviewer, admin).
type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code        string    `gorm:"uniqueIndex;not null"`
	Name        string    `gorm:"not null"`
	Description string
	CreatedAt   time.Time
}

// User represents a platform user with exactly one role.
type User struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name              string    `gorm:"not null"`
	Phone             string
	Email             string    `gorm:"type:citext;uniqueIndex;not null"`
	PasswordHash      string    `gorm:"not null"`
	PasswordUpdatedAt time.Time `gorm:"not null"`
	RoleID            uuid.UUID `gorm:"type:uuid;not null;index"`
	Role              Role      `gorm:"foreignKey:RoleID;references:ID"`
	Status            string    `gorm:"not null;default:active"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TableName overrides GORM pluralization for clarity.
func (Role) TableName() string { return "roles" }
func (User) TableName() string { return "users" }
