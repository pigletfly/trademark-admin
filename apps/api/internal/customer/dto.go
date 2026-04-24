package customer

import (
	"time"

	"github.com/google/uuid"
)

// CustomerDTO is the wire representation of a customer row.
type CustomerDTO struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Industry       *string   `json:"industry,omitempty"`
	IsReturning    bool      `json:"is_returning"`
	PriceSensitive bool      `json:"price_sensitive"`
	ContactName    *string   `json:"contact_name,omitempty"`
	ContactPhone   *string   `json:"contact_phone,omitempty"`
	ContactEmail   *string   `json:"contact_email,omitempty"`
	Notes          *string   `json:"notes,omitempty"`
	CreatedBy      uuid.UUID `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ListResponse is the paginated list envelope returned to clients.
type ListResponse struct {
	Items    []CustomerDTO `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}

// CreateRequest — client-submitted body for POST /customers.
type CreateRequest struct {
	Name           string  `json:"name" binding:"required,min=1,max=200"`
	Industry       *string `json:"industry,omitempty"`
	IsReturning    bool    `json:"is_returning"`
	PriceSensitive bool    `json:"price_sensitive"`
	ContactName    *string `json:"contact_name,omitempty"`
	ContactPhone   *string `json:"contact_phone,omitempty"`
	ContactEmail   *string `json:"contact_email,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

// UpdateRequest — client-submitted body for PATCH /customers/:id.
// All fields optional; only present (non-nil) fields are applied.
type UpdateRequest struct {
	Name           *string `json:"name,omitempty" binding:"omitempty,min=1,max=200"`
	Industry       *string `json:"industry,omitempty"`
	IsReturning    *bool   `json:"is_returning,omitempty"`
	PriceSensitive *bool   `json:"price_sensitive,omitempty"`
	ContactName    *string `json:"contact_name,omitempty"`
	ContactPhone   *string `json:"contact_phone,omitempty"`
	ContactEmail   *string `json:"contact_email,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

func toDTO(c Customer) CustomerDTO {
	return CustomerDTO{
		ID:             c.ID,
		Name:           c.Name,
		Industry:       c.Industry,
		IsReturning:    c.IsReturning,
		PriceSensitive: c.PriceSensitive,
		ContactName:    c.ContactName,
		ContactPhone:   c.ContactPhone,
		ContactEmail:   c.ContactEmail,
		Notes:          c.Notes,
		CreatedBy:      c.CreatedBy,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}
