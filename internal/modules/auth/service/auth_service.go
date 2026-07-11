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

const bcryptCost = 12

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
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
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
	// Brute-force / account lockout check
	if err := s.checkLoginAttempts(ctx, req.Email, ipAddress); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil || user == nil {
		s.recordFailedAttempt(ctx, req.Email, ipAddress)
		return nil, fmt.Errorf("invalid email or password")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.recordFailedAttempt(ctx, req.Email, ipAddress)
		return nil, fmt.Errorf("invalid email or password")
	}

	// Clear failed attempts on success
	s.redis.Del(ctx, fmt.Sprintf("login_attempts:%s", req.Email))
	s.redis.Del(ctx, fmt.Sprintf("login_lockout:%s", req.Email))

	return s.generateTokens(ctx, user, deviceID, ipAddress, userAgent)
}

func (s *AuthService) checkLoginAttempts(ctx context.Context, email, ip string) error {
	lockoutKey := fmt.Sprintf("login_lockout:%s", email)
	locked, _ := s.redis.Exists(ctx, lockoutKey).Result()
	if locked > 0 {
		ttl, _ := s.redis.TTL(ctx, lockoutKey).Result()
		return fmt.Errorf("account locked. Try again in %d minutes", int(ttl.Minutes())+1)
	}
	return nil
}

func (s *AuthService) recordFailedAttempt(ctx context.Context, email, ip string) {
	key := fmt.Sprintf("login_attempts:%s", email)
	attempts, _ := s.redis.Incr(ctx, key).Result()
	s.redis.Expire(ctx, key, 15*time.Minute)

	if attempts >= 5 {
		lockoutKey := fmt.Sprintf("login_lockout:%s", email)
		s.redis.Set(ctx, lockoutKey, "locked", 15*time.Minute)
		logger.Warn("account locked due to failed attempts", zap.String("email", email), zap.String("ip", ip))
	}
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
			tokenHash := hashKey(accessToken)
			key := fmt.Sprintf("blacklist:%s", tokenHash)
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

	_, err = s.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, device_id, ip_address, expires_at) VALUES ($1, $2, $3, $4::inet, $5)`,
		user.ID, hashed, deviceID, ipAddress, refreshExp)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

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
	// Single SQL: delete sessions beyond the limit, keeping the newest N
	_, err := s.db.Exec(ctx,
		`DELETE FROM sessions WHERE user_id=$1 AND id NOT IN (
			SELECT id FROM sessions WHERE user_id=$1 ORDER BY last_accessed_at DESC LIMIT $2
		)`, userID, maxSessions)
	if err != nil {
		logger.Error("failed to enforce session limit", zap.Error(err))
	}
	// Also clean up orphaned refresh tokens
	_, err = s.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id=$1 AND token_hash NOT IN (
			SELECT refresh_token_hash FROM sessions WHERE user_id=$1
		) AND expires_at > now()`, userID)
	if err != nil {
		logger.Error("failed to cleanup orphaned refresh tokens", zap.Error(err))
	}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
