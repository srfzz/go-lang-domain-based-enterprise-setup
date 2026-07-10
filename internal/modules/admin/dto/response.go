package dto

import "time"

type UserResponse struct {
	ID        string       `json:"id"`
	Email     string       `json:"email"`
	FullName  string       `json:"full_name"`
	IsActive  bool         `json:"is_active"`
	Roles     []RoleInfo   `json:"roles,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

type RoleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type RoleResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Category    string              `json:"category"`
	Description string              `json:"description"`
	Permissions []PermissionInfo    `json:"permissions,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

type PermissionInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
}

type PermissionResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
