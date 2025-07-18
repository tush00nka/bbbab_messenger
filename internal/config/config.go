package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	DSN  string `mapstructure:"DB_DSN"`
	Port string `mapstructure:"SERVER_PORT"`
}

func Load() (*Config, error) {
	viper.AddConfigPath("./")
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.DSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}

	return &cfg, nil
}
