package config

import (
	"log"
	"strings"

	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type DatabaseConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	User    string `mapstructure:"user"`
	Pass    string `mapstructure:"pass"`
	Name    string `mapstructure:"name"`
	SSLMode string `mapstructure:"sslmode"`
}

type AuthConfig struct {
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	SecretCookie string `mapstructure:"secret_cookie"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.Auth.SecretCookie == "" {
		secretCookie, err := middleware.GenerateRandomHex(32)
		log.Println("Generated secret cookie")
		if err != nil {
			return nil, err
		}
		cfg.Auth.SecretCookie = secretCookie
	} else {
		log.Println("Using secret cookie from config")
	}

	if cfg.Auth.Username == "" || cfg.Auth.Password == "" {
		if cfg.Auth.Username == "" {
			cfg.Auth.Username = "admin"
			log.Println("Generated username")
		}

		if cfg.Auth.Password == "" {
			password, err := middleware.GenerateRandomHex(16)
			log.Println("Generated password")
			if err != nil {
				return nil, err
			}
			cfg.Auth.Password = password
		}

		log.Printf("Generated credentials: %s:%s\n", cfg.Auth.Username, cfg.Auth.Password)
	} else {
		log.Println("Using credentials from config")
	}

	return &cfg, nil
}
