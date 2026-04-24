package auth

import "github.com/google/uuid"

// LoginRequest models the POST /auth/login body.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=1"`
}

// UserResponse is the slimmed-down representation of a user returned to clients.
type UserResponse struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Phone  string    `json:"phone,omitempty"`
	Role   string    `json:"role"`   // role code
	Status string    `json:"status"`
}

// ToResponse converts a User into its API-facing shape.
func ToResponse(u *User) UserResponse {
	return UserResponse{
		ID:     u.ID,
		Name:   u.Name,
		Email:  u.Email,
		Phone:  u.Phone,
		Role:   u.Role.Code,
		Status: u.Status,
	}
}

// CreateUserRequest models POST /admin/users body.
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	RoleCode string `json:"role" binding:"required,oneof=salesperson reviewer admin"`
	Password string `json:"password" binding:"required,min=8"`
}

// UpdateUserRequest models PATCH /admin/users/{id} body. All fields optional.
type UpdateUserRequest struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	RoleCode *string `json:"role" binding:"omitempty,oneof=salesperson reviewer admin"`
	Status   *string `json:"status" binding:"omitempty,oneof=active disabled"`
}

// ResetPasswordResponse is returned from the admin reset endpoint.
// Contains the freshly generated password — admin is expected to deliver it OOB.
type ResetPasswordResponse struct {
	Password string `json:"password"`
}
