package dto

import "github.com/google/uuid"

type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required,email"`
	Password string   `json:"password" binding:"required,min=8"`
	FullName string   `json:"full_name" binding:"required"`
	RoleIDs  []string `json:"role_ids"`
}

type AssignRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids" binding:"required"`
}

type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type CreatePermissionRequest struct {
	Name        string `json:"name" binding:"required"`
	Action      string `json:"action" binding:"required"`
	Resource    string `json:"resource" binding:"required"`
	Description string `json:"description"`
}

type UpdatePermissionRequest struct {
	Name        string `json:"name"`
	Action      string `json:"action"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
}

type AssignPermissionRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids" binding:"required"`
}
