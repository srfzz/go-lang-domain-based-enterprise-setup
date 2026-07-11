package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/modules/admin/domain"
	"github.com/yourorg/enterprise-api/internal/modules/admin/dto"
	"github.com/yourorg/enterprise-api/internal/modules/admin/repository"
	authdomain "github.com/yourorg/enterprise-api/internal/modules/auth/domain"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"go.uber.org/zap"
)

type AdminService struct {
	userRepo *repository.AdminUserRepository
	roleRepo *repository.RoleRepository
	permRepo *repository.PermissionRepository
	db       *pgxpool.Pool
	redis    *redis.Client
	cfg      *config.Config
}

func NewAdminService(db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) *AdminService {
	return &AdminService{
		userRepo: repository.NewAdminUserRepository(db),
		roleRepo: repository.NewRoleRepository(db),
		permRepo: repository.NewPermissionRepository(db),
		db:       db,
		redis:    redisClient,
		cfg:      cfg,
	}
}

// SeedDefaultAdmin creates the admin user, admin role, and base permissions if they don't exist.
func (s *AdminService) SeedDefaultAdmin(ctx context.Context) {
	adminEmail := s.cfg.AdminEmail
	if adminEmail == "" {
		adminEmail = "admin@enterprise.com"
	}
	adminPassword := s.cfg.AdminPassword
	if adminPassword == "" {
		adminPassword = "Admin123!"
	}
	adminName := s.cfg.AdminName
	if adminName == "" {
		adminName = "System Admin"
	}

	existing, err := s.userRepo.FindByEmail(ctx, adminEmail)
	if err != nil {
		logger.Error("failed to check for existing admin", zap.Error(err))
		return
	}
	if existing != nil {
		logger.Info("default admin already exists, skipping seed")
		return
	}

	// Ensure admin role exists
	adminRole, err := s.roleRepo.FindByName(ctx, "admin")
	if err != nil {
		logger.Error("failed to check for admin role", zap.Error(err))
		return
	}
	if adminRole == nil {
		adminRole = &domain.Role{Name: "admin", Category: "Administration", Description: "Full system access"}
		if err := s.roleRepo.Create(ctx, adminRole); err != nil {
			logger.Error("failed to create admin role", zap.Error(err))
			return
		}
		logger.Info("created admin role")
	}

	// Ensure all base permissions exist and assign to admin role
	s.seedPermissions(ctx, adminRole.ID)

	// Create admin user
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 12)
	if err != nil {
		logger.Error("failed to hash admin password", zap.Error(err))
		return
	}
	adminUser := &authdomain.User{
		Email:        adminEmail,
		PasswordHash: string(hash),
		FullName:     adminName,
	}
	if err := s.userRepo.Create(ctx, adminUser); err != nil {
		logger.Error("failed to create admin user", zap.Error(err))
		return
	}

	// Assign admin role
	if err := s.userRepo.AssignRole(ctx, adminUser.ID, adminRole.ID); err != nil {
		logger.Error("failed to assign admin role", zap.Error(err))
		return
	}

	logger.Info("default admin seeded",
		zap.String("email", adminEmail),
		zap.String("name", adminName),
	)
}

func (s *AdminService) seedPermissions(ctx context.Context, adminRoleID uuid.UUID) {
	basePermissions := []struct {
		Name     string
		Action   string
		Resource string
		Desc     string
	}{
		{"Admin Access", "manage", "admin", "Access the admin panel"},
		{"Create Incident", "create", "incident", "Create new incidents"},
		{"Read Incident", "read", "incident", "View incidents"},
		{"Update Incident", "update", "incident", "Update incidents"},
		{"Delete Incident", "delete", "incident", "Delete incidents"},
		{"Manage Users", "manage", "users", "Create and manage users"},
		{"Manage Roles", "manage", "roles", "Create and manage roles"},
		{"Manage Permissions", "manage", "permissions", "Create and manage permissions"},
	}

	var permIDs []uuid.UUID
	for _, bp := range basePermissions {
		p := &domain.Permission{
			Name:        bp.Name,
			Action:      bp.Action,
			Resource:    bp.Resource,
			Description: bp.Desc,
		}
		if err := s.permRepo.Create(ctx, p); err != nil {
			logger.Warn("permission may already exist, skipping", zap.String("name", bp.Name))
			continue
		}
		permIDs = append(permIDs, p.ID)
	}

	if len(permIDs) > 0 {
		if err := s.roleRepo.AssignPermissions(ctx, adminRoleID, permIDs); err != nil {
			logger.Error("failed to assign permissions to admin role", zap.Error(err))
		}
	}
}

// --- User management ---

func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]dto.UserResponse, int, error) {
	users, err := s.userRepo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var resp []dto.UserResponse
	for _, u := range users {
		roles, _ := s.userRepo.GetRoles(ctx, u.ID)
		var roleInfos []dto.RoleInfo
		for _, r := range roles {
			roleInfos = append(roleInfos, dto.RoleInfo{
				ID:          r.ID.String(),
				Name:        r.Name,
				Category:    r.Category,
				Description: r.Description,
			})
		}
		resp = append(resp, dto.UserResponse{
			ID:        u.ID.String(),
			Email:     u.Email,
			FullName:  u.FullName,
			IsActive:  u.IsActive,
			Roles:     roleInfos,
			CreatedAt: u.CreatedAt,
		})
	}
	return resp, total, nil
}

func (s *AdminService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	existing, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return nil, fmt.Errorf("email already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	user := &authdomain.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     req.FullName,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign roles if provided
	for _, roleIDStr := range req.RoleIDs {
		rid, err := uuid.Parse(roleIDStr)
		if err != nil {
			continue
		}
		s.userRepo.AssignRole(ctx, user.ID, rid)
	}

	roles, _ := s.userRepo.GetRoles(ctx, user.ID)
	var roleInfos []dto.RoleInfo
	for _, r := range roles {
		roleInfos = append(roleInfos, dto.RoleInfo{
			ID:          r.ID.String(),
			Name:        r.Name,
			Category:    r.Category,
			Description: r.Description,
		})
	}

	return &dto.UserResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		FullName:  user.FullName,
		IsActive:  user.IsActive,
		Roles:     roleInfos,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *AdminService) AssignRoles(ctx context.Context, userID uuid.UUID, req dto.AssignRolesRequest) error {
	return s.userRepo.SetRoles(ctx, userID, req.RoleIDs)
}

func (s *AdminService) GetUser(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error) {
	u, err := s.userRepo.FindByID(ctx, id)
	if err != nil || u == nil {
		return nil, fmt.Errorf("user not found")
	}
	roles, _ := s.userRepo.GetRoles(ctx, u.ID)
	var roleInfos []dto.RoleInfo
	for _, r := range roles {
		roleInfos = append(roleInfos, dto.RoleInfo{
			ID:          r.ID.String(),
			Name:        r.Name,
			Category:    r.Category,
			Description: r.Description,
		})
	}
	return &dto.UserResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		FullName:  u.FullName,
		IsActive:  u.IsActive,
		Roles:     roleInfos,
		CreatedAt: u.CreatedAt,
	}, nil
}

// --- Role management ---

func (s *AdminService) ListRoles(ctx context.Context, limit, offset int) ([]dto.RoleResponse, int, error) {
	roles, err := s.roleRepo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.roleRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var resp []dto.RoleResponse
	for _, r := range roles {
		perms, _ := s.roleRepo.GetPermissions(ctx, r.ID)
		var permInfos []dto.PermissionInfo
		for _, p := range perms {
			permInfos = append(permInfos, dto.PermissionInfo{
				ID:          p.ID.String(),
				Name:        p.Name,
				Action:      p.Action,
				Resource:    p.Resource,
				Description: p.Description,
			})
		}
		resp = append(resp, dto.RoleResponse{
			ID:          r.ID.String(),
			Name:        r.Name,
			Category:    r.Category,
			Description: r.Description,
			Permissions: permInfos,
			CreatedAt:   r.CreatedAt,
		})
	}
	return resp, total, nil
}

func (s *AdminService) CreateRole(ctx context.Context, req dto.CreateRoleRequest) (*dto.RoleResponse, error) {
	role := &domain.Role{
		Name:        req.Name,
		Category:    req.Category,
		Description: req.Description,
	}
	if role.Category == "" {
		role.Category = "general"
	}
	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}
	return &dto.RoleResponse{
		ID:          role.ID.String(),
		Name:        role.Name,
		Category:    role.Category,
		Description: role.Description,
		CreatedAt:   role.CreatedAt,
	}, nil
}

func (s *AdminService) UpdateRole(ctx context.Context, id uuid.UUID, req dto.UpdateRoleRequest) (*dto.RoleResponse, error) {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil || role == nil {
		return nil, fmt.Errorf("role not found")
	}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Category != "" {
		role.Category = req.Category
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	if err := s.roleRepo.Update(ctx, id, role); err != nil {
		return nil, err
	}
	return &dto.RoleResponse{
		ID:          role.ID.String(),
		Name:        role.Name,
		Category:    role.Category,
		Description: role.Description,
		CreatedAt:   role.CreatedAt,
	}, nil
}

func (s *AdminService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return s.roleRepo.Delete(ctx, id)
}

func (s *AdminService) AssignPermissions(ctx context.Context, roleID uuid.UUID, req dto.AssignPermissionRequest) error {
	return s.roleRepo.AssignPermissions(ctx, roleID, req.PermissionIDs)
}

// --- Permission management ---

func (s *AdminService) ListPermissions(ctx context.Context, limit, offset int) ([]dto.PermissionResponse, int, error) {
	perms, err := s.permRepo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.permRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var resp []dto.PermissionResponse
	for _, p := range perms {
		resp = append(resp, dto.PermissionResponse{
			ID:          p.ID.String(),
			Name:        p.Name,
			Action:      p.Action,
			Resource:    p.Resource,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
		})
	}
	return resp, total, nil
}

func (s *AdminService) CreatePermission(ctx context.Context, req dto.CreatePermissionRequest) (*dto.PermissionResponse, error) {
	p := &domain.Permission{
		Name:        req.Name,
		Action:      req.Action,
		Resource:    req.Resource,
		Description: req.Description,
	}
	if err := s.permRepo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}
	return &dto.PermissionResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Action:      p.Action,
		Resource:    p.Resource,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}, nil
}

func (s *AdminService) UpdatePermission(ctx context.Context, id uuid.UUID, req dto.UpdatePermissionRequest) (*dto.PermissionResponse, error) {
	p, err := s.permRepo.FindByID(ctx, id)
	if err != nil || p == nil {
		return nil, fmt.Errorf("permission not found")
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Action != "" {
		p.Action = req.Action
	}
	if req.Resource != "" {
		p.Resource = req.Resource
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if err := s.permRepo.Update(ctx, id, p); err != nil {
		return nil, err
	}
	return &dto.PermissionResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Action:      p.Action,
		Resource:    p.Resource,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}, nil
}

func (s *AdminService) DeletePermission(ctx context.Context, id uuid.UUID) error {
	return s.permRepo.Delete(ctx, id)
}
