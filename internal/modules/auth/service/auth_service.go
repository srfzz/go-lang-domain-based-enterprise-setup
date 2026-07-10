package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourorg/enterprise-api/internal/config"
	"github.com/yourorg/enterprise-api/internal/modules/auth/domain"
	"github.com/yourorg/enterprise-api/internal/modules/auth/dto"
	"github.com/yourorg/enterprise-api/internal/modules/auth/repository"
	"github.com/yourorg/enterprise-api/internal/shared/logger"
	"github.com/yourorg/enterprise-api/internal/shared/utils"
	"go.uber.org/zap"
)

type AuthService struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	db          *pgxpool.Pool
	redis       *redis.Client
	cfg         *config.Config
}

func NewAuthService(db *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:    repository.NewUserRepository(db),
		sessionRepo: repository.NewSessionRepository(db),
		db:          db,
		redis:       redisClient,
		cfg:         cfg,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     req.FullName,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("email already exists or db error: %w", err)
	}
	return s.generateTokens(ctx, user, "register_device", "", "")
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, deviceID, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil || user == nil {
		return nil, fmt.Errorf("invalid email or password")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}
	return s.generateTokens(ctx, user, deviceID, ipAddress, userAgent)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken, deviceID, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	hashed := hashToken(refreshToken)

	var token domain.RefreshToken
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, device_id, expires_at FROM refresh_tokens WHERE token_hash=$1`,
		hashed).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.DeviceID, &token.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	if time.Now().After(token.ExpiresAt) {
		s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE id=$1`, token.ID)
		return nil, fmt.Errorf("refresh token expired")
	}

	// Remove the old session and refresh token (rotation)
	s.sessionRepo.DeleteByRefreshHash(ctx, hashed)
	s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE id=$1`, token.ID)

	user, err := s.userRepo.FindByID(ctx, token.UserID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return s.generateTokens(ctx, user, deviceID, ipAddress, userAgent)
}

func (s *AuthService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := utils.ValidateAccessToken(accessToken)
	if err == nil {
		remaining := time.Until(claims.ExpiresAt.Time)
		if remaining > 0 {
			key := fmt.Sprintf("blacklist:access:%s", accessToken)
			s.redis.Set(ctx, key, "revoked", remaining)
		}
	}
	if refreshToken != "" {
		hashed := hashToken(refreshToken)
		s.sessionRepo.DeleteByRefreshHash(ctx, hashed)
		s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, hashed)
	}
	return nil
}

func (s *AuthService) generateTokens(ctx context.Context, user *domain.User, deviceID, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	accessToken, accessExp, err := utils.GenerateAccessToken(user.ID, user.Email, time.Duration(s.cfg.JWTAccessExpiryMin)*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTokenStr, refreshExp, err := utils.GenerateRefreshToken(user.ID, time.Duration(s.cfg.JWTRefreshExpiryDays)*24*time.Hour)
	if err != nil {
		return nil, err
	}

	hashed := hashToken(refreshTokenStr)

	// Store refresh token
	_, err = s.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, device_id, ip_address, expires_at) VALUES ($1, $2, $3, $4::inet, $5)`,
		user.ID, hashed, deviceID, ipAddress, refreshExp)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Create session
	session := &domain.Session{
		UserID:           user.ID,
		RefreshTokenHash: hashed,
		DeviceID:         deviceID,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		logger.Error("failed to create session", zap.Error(err))
	}

	// Enforce max active sessions — evict oldest if over limit
	s.enforceSessionLimit(ctx, user.ID)

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    accessExp,
		User: dto.UserInfo{
			ID:       user.ID.String(),
			Email:    user.Email,
			FullName: user.FullName,
		},
	}, nil
}

func (s *AuthService) enforceSessionLimit(ctx context.Context, userID uuid.UUID) {
	maxSessions := s.cfg.MaxActiveSessions
	if maxSessions <= 0 {
		return
	}
	for {
		count, err := s.sessionRepo.CountByUserID(ctx, userID)
		if err != nil || count <= maxSessions {
			break
		}
		oldest, err := s.sessionRepo.FindOldestByUserID(ctx, userID)
		if err != nil || oldest == nil {
			break
		}
		// Delete the oldest session and its refresh token
		if err := s.sessionRepo.DeleteByID(ctx, oldest.ID); err != nil {
			logger.Error("failed to evict oldest session", zap.Error(err))
			break
		}
		s.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, oldest.RefreshTokenHash)
		logger.Info("evicted oldest session",
			zap.String("user_id", userID.String()),
			zap.String("evicted_session_id", oldest.ID.String()),
		)
	}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
