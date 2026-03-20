package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/SelfScriptKiddies/tweaker/internal/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Log       LogConfig       `yaml:"log"`
	Auth      AuthConfig      `yaml:"auth"`
	Files     FilesConfig     `yaml:"files"`
	Shells    ShellsConfig    `yaml:"shells"`
	Templates TemplatesConfig `yaml:"templates"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	Env   string `yaml:"env"`
}

type AuthConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	SecretCookie string `yaml:"secret_cookie"`
}

type FilesConfig struct {
	Directory string `yaml:"directory"`
}

type ShellsConfig struct {
	ListenPort int `yaml:"listen_port"`
}

type TemplatesConfig struct {
	File string `yaml:"file"`
}

const defaultConfigYAML = `server:
  host: "0.0.0.0"
  port: 8080

log:
  level: "info"          # debug, info, warn, error
  env: "local"           # local, prod

auth:
  username: "admin"
  password: ""           # auto-generated if empty (printed to stdout)
  secret_cookie: ""      # auto-generated if empty

files:
  directory: "./files"   # file storage directory

shells:
  listen_port: 4444      # TCP port for reverse shell connections

templates:
  file: "templates.yaml" # command templates file (created on first run)
`

func Generate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	return os.WriteFile(path, []byte(defaultConfigYAML), 0644)
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
		Log:    LogConfig{Level: "info", Env: "local"},
		Auth:   AuthConfig{Username: "admin"},
		Files:  FilesConfig{Directory: "./files"},
		Shells:    ShellsConfig{ListenPort: 4444},
		Templates: TemplatesConfig{File: "templates.yaml"},
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("config parse error: %w", err)
			}
		}
	}

	if cfg.Auth.SecretCookie == "" {
		cookie, err := middleware.GenerateRandomHex(32)
		if err != nil {
			return nil, err
		}
		cfg.Auth.SecretCookie = cookie
	}

	if cfg.Auth.Password == "" {
		password, err := middleware.GenerateRandomHex(16)
		if err != nil {
			return nil, err
		}
		cfg.Auth.Password = password
		fmt.Printf("Generated credentials: %s:%s\n", cfg.Auth.Username, cfg.Auth.Password)
	}

	return cfg, nil
}

func InitLogger(logCfg LogConfig) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set(strings.ToLower(logCfg.Level)); err != nil {
		return nil, err
	}

	if strings.ToLower(logCfg.Env) == "prod" {
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(level)
		return cfg.Build()
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("01.01 15:04:05"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
