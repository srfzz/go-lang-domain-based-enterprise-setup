package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	AppName  string
	AppEnv   string
	AppPort  string
	AppDebug bool

	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBSSLMode      string
	DBMaxOpenConns int
	DBMaxIdleConns int

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	JWTPrivateKeyPath    string
	JWTPublicKeyPath     string
	JWTAccessExpiryMin   int
	JWTRefreshExpiryDays int

	RateLimitRequests    int
	RateLimitDurationSec int
	ThrottleBurst        int
	ThrottleRate         int

	LogLevel    string
	LogFilePath string

	StorageDriver   string
	StorageLocalPath string
	StorageS3Endpoint string
	StorageS3Region  string
	StorageS3Bucket  string
	StorageS3AccessKey string
	StorageS3SecretKey string
	StorageS3UseSSL   bool

	MaxActiveSessions int
}

func Load() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()

	cfg := &Config{
		AppName:             viper.GetString("APP_NAME"),
		AppEnv:              viper.GetString("APP_ENV"),
		AppPort:             viper.GetString("APP_PORT"),
		AppDebug:            viper.GetBool("APP_DEBUG"),
		DBHost:              viper.GetString("DB_HOST"),
		DBPort:              viper.GetString("DB_PORT"),
		DBUser:              viper.GetString("DB_USER"),
		DBPassword:          viper.GetString("DB_PASSWORD"),
		DBName:              viper.GetString("DB_NAME"),
		DBSSLMode:           viper.GetString("DB_SSLMODE"),
		DBMaxOpenConns:      viper.GetInt("DB_MAX_OPEN_CONNS"),
		DBMaxIdleConns:      viper.GetInt("DB_MAX_IDLE_CONNS"),
		RedisHost:           viper.GetString("REDIS_HOST"),
		RedisPort:           viper.GetString("REDIS_PORT"),
		RedisPassword:       viper.GetString("REDIS_PASSWORD"),
		RedisDB:             viper.GetInt("REDIS_DB"),
		JWTPrivateKeyPath:   viper.GetString("JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:    viper.GetString("JWT_PUBLIC_KEY_PATH"),
		JWTAccessExpiryMin:  viper.GetInt("JWT_ACCESS_EXPIRY_MINUTES"),
		JWTRefreshExpiryDays: viper.GetInt("JWT_REFRESH_EXPIRY_DAYS"),
		RateLimitRequests:   viper.GetInt("RATE_LIMIT_REQUESTS"),
		RateLimitDurationSec: viper.GetInt("RATE_LIMIT_DURATION_SECONDS"),
		ThrottleBurst:       viper.GetInt("THROTTLE_BURST"),
		ThrottleRate:        viper.GetInt("THROTTLE_RATE"),
		LogLevel:            viper.GetString("LOG_LEVEL"),
		LogFilePath:         viper.GetString("LOG_FILE_PATH"),
		StorageDriver:       viper.GetString("STORAGE_DRIVER"),
		StorageLocalPath:    viper.GetString("STORAGE_LOCAL_PATH"),
		StorageS3Endpoint:   viper.GetString("STORAGE_S3_ENDPOINT"),
		StorageS3Region:     viper.GetString("STORING_S3_REGION"),
		StorageS3Bucket:     viper.GetString("STORAGE_S3_BUCKET"),
		StorageS3AccessKey:  viper.GetString("STORAGE_S3_ACCESS_KEY"),
		StorageS3SecretKey:  viper.GetString("STORAGE_S3_SECRET_KEY"),
		StorageS3UseSSL:     viper.GetBool("STORAGE_S3_USE_SSL"),
		MaxActiveSessions:   viper.GetInt("MAX_ACTIVE_SESSIONS"),
	}

	if cfg.MaxActiveSessions <= 0 {
		cfg.MaxActiveSessions = 2
	}
	if cfg.JWTPrivateKeyPath == "" || cfg.JWTPublicKeyPath == "" {
		return nil, fmt.Errorf("JWT key paths must be set")
	}
	return cfg, nil
}
