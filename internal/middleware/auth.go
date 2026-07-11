package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/yourorg/enterprise-api/internal/shared/utils"
)

var tokenCache = utils.NewTokenCache(5*time.Minute, 10000)

func AuthRequired(redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		tokenHash := hashKey(tokenStr)

		// Check in-memory cache first (avoids RSA verify + Redis round-trip)
		if cached := tokenCache.Get(tokenHash); cached != nil {
			if claims, ok := cached.(*utils.AccessClaims); ok {
				c.Set("userID", claims.UserID)
				c.Set("userEmail", claims.Email)
				c.Next()
				return
			}
		}

		// Verify RSA signature
		claims, err := utils.ValidateAccessToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Check Redis blacklist (hash the token for key safety)
		blacklisted, err := redisClient.Exists(c.Request.Context(), "blacklist:"+tokenHash).Result()
		if err == nil && blacklisted > 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}

		// Cache the valid claims
		tokenCache.Set(tokenHash, claims)

		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Next()
	}
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
