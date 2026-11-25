package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	MasterAPIKey   string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	MediaBodyLimit string
}

func LoadConfig() *Config {
	if err := godotenv.Load(".env"); err != nil {
		if errParent := godotenv.Load("../.env"); errParent != nil {
			log.Println("No .env file found, relying on environment variables")
		}
	}

	masterKey := strings.TrimSpace(getEnv("MASTER_API_KEY", ""))
	if masterKey == "" {
		log.Fatal("❌ MASTER_API_KEY environment variable is required and cannot be empty")
	}

	return &Config{
		Port:           getEnv("PORT", "3000"),
		MasterAPIKey:   masterKey,
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "root"),
		DBPassword:     getEnv("DB_PASSWORD", "rootpassword"),
		DBName:         getEnv("DB_NAME", "whatsapp_bot"),
		MediaBodyLimit: getEnv("MEDIA_BODY_LIMIT", "25mb"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
