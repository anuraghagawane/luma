// Package config load the environment variables
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ElasticBroker string
	KafkaBroker   string
	CollectorPort string
	QueryPort     string
	PostgresDBUrl string
	JwtSecret     string
	TokenLifetime int
}

func LoadEnv() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	tokenLifeTime, err := strconv.Atoi(os.Getenv("JWT_TOKEN_LIFE_TIME"))
	if err != nil {
		tokenLifeTime = 10
	}

	return &Config{
		ElasticBroker: os.Getenv("ELASTIC_BROKER"),
		KafkaBroker:   os.Getenv("KAFKA_BROKER"),
		CollectorPort: os.Getenv("COLLECTOR_PORT"),
		QueryPort:     os.Getenv("QUERY_PORT"),
		PostgresDBUrl: os.Getenv("POSTGRES_DB_URL"),
		JwtSecret:     os.Getenv("JWT_SECRET"),
		TokenLifetime: tokenLifeTime,
	}, nil
}
