package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName     string
	AppEnv      string
	Port        string
	CorsOrigins string

	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPass     string
	DBName     string
	DBSSL      string
	DBTimezone string
	DBPath     string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string

	LLMAPIKey     string
	LLMModel      string
	WebhookSecret string
}

func LoadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found or provided by environment")
	}

	return Config{
		AppName:     getEnv("APP_NAME", "Kanso API"),
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		CorsOrigins: getEnv("CORS_ALLOW_ORIGINS", "*"),

		DBDriver:   getEnv("DB_DRIVER", "sqlite"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPass:     getEnv("DB_PASSWORD", "postgres123"),
		DBName:     getEnv("DB_NAME", "kanso_db"),
		DBSSL:      getEnv("DB_SSLMODE", "disable"),
		DBTimezone: getEnv("DB_TIMEZONE", "Asia/Jakarta"),
		DBPath:     getEnv("DB_PATH", "kanso.db"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/duo/auth/google/callback"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),

		LLMAPIKey:          getEnv("LLM_API_KEY", getEnv("GEMINI_API_KEY", "")),
		LLMModel:           getEnv("LLM_MODEL", "gemini-2.5-flash-lite"),
		WebhookSecret:      getEnv("WEBHOOK_SECRET", "kanso_webhook_secret_key"),
	}
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		c.DBHost, c.DBUser, c.DBPass, c.DBName, c.DBPort, c.DBSSL, c.DBTimezone,
	)
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
