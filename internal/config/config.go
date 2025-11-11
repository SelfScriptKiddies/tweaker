package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	"github.com/ilyakaznacheev/cleanenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Log      LogConfig      `yaml:"log"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Host string `yaml:"host" env:"SERVER_HOST" default-env:"0.0.0.0"`
	Port int    `yaml:"port" env:"SERVER_PORT" default-env:"8080"`
}

type LogConfig struct {
	Level string `yaml:"level" env:"LOG_LEVEL" default-env:"info"`
	// Can be: local, dev, prod
	Env string `yaml:"env" env:"LOG_ENV" default-env:"dev"`
}

type DatabaseConfig struct {
	Host    string `yaml:"host" env:"DATABASE_HOST" default-env:"db"`
	Port    int    `yaml:"port" env:"DATABASE_PORT" default-env:"5432"`
	User    string `yaml:"user" env:"DATABASE_USERNAME" default-env:"postgres"`
	Pass    string `yaml:"pass" env:"DATABASE_PASSWORD" default-env:"postgres"`
	Name    string `yaml:"name" env:"DATABASE_SCHEME" default-env:"tweaker"`
	SSLMode string `yaml:"sslmode" env:"DATABASE_USER" default-env:"postgres"`
}

type AuthConfig struct {
	Username     string `yaml:"username" env:"WEB_USERNAME" default-env:"admin"`
	Password     string `yaml:"password" env:"WEB_PASSWORD"`
	SecretCookie string `yaml:"secret_cookie" env:"SECRET_COOKIE"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, err
	}

	if cfg.Auth.SecretCookie == "" {
		secretCookie, err := middleware.GenerateRandomHex(32)
		fmt.Println("Generated secret cookie")
		if err != nil {
			return nil, err
		}
		cfg.Auth.SecretCookie = secretCookie
	}

	if cfg.Auth.Password == "" {
		password, err := middleware.GenerateRandomHex(16)
		if err != nil {
			return nil, err
		}
		cfg.Auth.Password = password
		fmt.Printf("Generated credentials: %s:%s\n", cfg.Auth.Username, cfg.Auth.Password)
	}

	return &cfg, nil
}

func InitLogger(log_config LogConfig) (*zap.Logger, error) {
	// Default level is info
	level := zapcore.InfoLevel
	if err := level.Set(strings.ToLower(log_config.Level)); err != nil {
		return nil, err
	}

	env := strings.ToLower(log_config.Env)
	if env == "prod" {
		// Production config - JSON format, no colors
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(level)
		return cfg.Build()
	}

	// Development/Local config - colorful console output
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,              // Color encoding for levels
		EncodeTime:     zapcore.TimeEncoderOfLayout("01.01 15:04:05"), // Short time format HH:MM:SS
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // Short caller format (file:line)
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return logger, nil
}
