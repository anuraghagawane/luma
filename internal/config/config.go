// Package config load the environment variables
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ElasticBroker string
	KafkaBroker   string
}

func LoadEnv() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	return &Config{
		ElasticBroker: os.Getenv("ELASTIC_BROKER"),
		KafkaBroker:   os.Getenv("KAFKA_BROKER"),
	}, nil
}
