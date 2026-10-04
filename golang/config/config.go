package config

import (
	"log"
	"os"

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
	err := godotenv.Load("../.env")
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	return &Config{
		Port:           getEnv("PORT", "3000"),
		MasterAPIKey:   getEnv("MASTER_API_KEY", "your_secure_master_api_key"),
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
