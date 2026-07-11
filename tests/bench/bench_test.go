package bench

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourorg/enterprise-api/internal/shared/utils"
)

var (
	testUserID   = uuid.New()
	testEmail    = "bench@test.com"
	accessToken  string
	refreshToken string
)

func TestMain(m *testing.M) {
	wd, _ := os.Getwd()
	privPath := wd + "/../../keys/private.pem"
	pubPath := wd + "/../../keys/public.pem"

	if err := utils.LoadKeys(privPath, pubPath); err != nil {
		fmt.Printf("Failed to load RSA keys: %v\n", err)
		os.Exit(1)
	}

	var err error
	accessToken, _, err = utils.GenerateAccessToken(testUserID, testEmail, 15*time.Minute)
	if err != nil {
		fmt.Printf("Failed to generate access token: %v\n", err)
		os.Exit(1)
	}
	refreshToken, _, err = utils.GenerateRefreshToken(testUserID, 7*24*time.Hour)
	if err != nil {
		fmt.Printf("Failed to generate refresh token: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// --- JWT Generation benchmarks ---

func BenchmarkGenerateAccessToken(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := utils.GenerateAccessToken(uuid.New(), testEmail, 15*time.Minute)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateRefreshToken(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := utils.GenerateRefreshToken(uuid.New(), 7*24*time.Hour)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- JWT Validation benchmarks ---

func BenchmarkValidateAccessToken(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.ValidateAccessToken(accessToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateAccessTokenParallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := utils.ValidateAccessToken(accessToken)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- Bcrypt benchmarks ---

func BenchmarkBcryptHash(b *testing.B) {
	password := "benchpassword123!"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBcryptCompare(b *testing.B) {
	password := "benchpassword123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
			b.Fatal(err)
		}
	}
}

// --- Token Cache benchmarks ---

func BenchmarkTokenCacheSet(b *testing.B) {
	cache := utils.NewTokenCache(5*time.Minute, 10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), "some-claims-data")
	}
}

func BenchmarkTokenCacheGet(b *testing.B) {
	cache := utils.NewTokenCache(5*time.Minute, 10000)
	for i := 0; i < 10000; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), "some-claims-data")
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(fmt.Sprintf("key-%d", i%10000))
			i++
		}
	})
}

// --- SHA-256 hashing benchmarks (used for token blacklist keys) ---

func BenchmarkSHA256(b *testing.B) {
	data := []byte("some-long-jwt-token-string-that-needs-hashing-for-blacklist")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := sha256.Sum256(data)
		_ = h[:]
	}
}

// --- UUID generation benchmark ---

func BenchmarkUUIDGeneration(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = uuid.New()
	}
}
