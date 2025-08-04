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

	return &cfg, nil
}
