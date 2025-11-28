package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Host     string `mapstructure:"DB_HOST"`
	User     string `mapstructure:"DB_USER"`
	Password string `mapstructure:"DB_PASSWORD"`
	Name     string `mapstructure:"DB_NAME"`
	DBPort   string `mapstructure:"DB_PORT"`

	ServerPort string `mapstructure:"SERVER_PORT"`

	SMSAPI string `mapstructure:"SMS_API"`

	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	S3Region          string `mapstructure:"S3_REGION"`
	S3AccessKeyID     string `mapstructure:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `mapstructure:"S3_SECRET_KEY_ACCESS"`
	S3BucketName      string `mapstructure:"S3_BUCKET_NAME"`
	S3Endpoint        string `mapstructure:"S3_ENDPOINT"`
	S3UseSSL          bool   `mapstructure:"S3_USE_SSL"`
}

func Load() (*Config, error) {
	viper.AddConfigPath("./")
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.User == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}

	if cfg.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}

	if cfg.DBPort == "" {
		return nil, fmt.Errorf("DB_PORT is required")
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("DB_HOST is required")
	}

	if cfg.ServerPort == "" {
		return nil, fmt.Errorf("SERVER_PORT is required")
	}

	if cfg.SMSAPI == "" {
		return nil, fmt.Errorf("SMS_API is required")
	}

	if cfg.RedisPort == "" {
		return nil, fmt.Errorf("REDIS_PORT is required")
	}

	if cfg.RedisPassword == "" {
		return nil, fmt.Errorf("REDIS_PASSWORD is required")
	}

	return &cfg, nil
}
