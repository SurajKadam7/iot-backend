package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	FrontendOrigin  string
	WebDistDir      string
	LogLevel        string
	LogFormat       string

	AuthMode     string
	JWTSecret    string
	JWTIssuer    string
	JWTAudience  string
	JWTJWKSURL   string
	JWTExpiry    time.Duration

	MQTTEnabled            bool
	MQTTBroker             string
	MQTTClientID           string
	MQTTUsername           string
	MQTTPassword           string
	MQTTCACertPath         string
	MQTTCertPath           string
	MQTTKeyPath            string
	MQTTInsecureSkipVerify bool
	MQTTKeepAlive          time.Duration

	MQTTSubClientID    string
	MQTTSubCACertPath  string
	MQTTSubCertPath    string
	MQTTSubKeyPath     string
}

func Load() (Config, error) {
	loadDotEnv(".env")

	cfg := Config{
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://iot:iot@127.0.0.1:5432/iot?sslmode=disable"),
		FrontendOrigin:  env("FRONTEND_ORIGIN", "http://localhost:5173"),
		WebDistDir:      env("WEB_DIST_DIR", "web/dist"),
		LogLevel:        env("LOG_LEVEL", "info"),
		LogFormat:       env("LOG_FORMAT", "text"),
		AuthMode:        strings.ToLower(env("AUTH_MODE", "local")),
		JWTSecret:       env("JWT_SECRET", ""),
		JWTIssuer:       env("JWT_ISSUER", "iot-local"),
		JWTAudience:     env("JWT_AUDIENCE", ""),
		JWTJWKSURL:      env("JWT_JWKS_URL", ""),
		JWTExpiry:       envDuration("JWT_EXPIRY", 24*time.Hour),
		MQTTEnabled:     envBool("MQTT_ENABLED", true),
		MQTTBroker:      env("MQTT_BROKER", "tcp://127.0.0.1:1883"),
		MQTTClientID:    env("MQTT_CLIENT_ID", "iot-backend-live"),
		MQTTUsername:    env("MQTT_USERNAME", ""),
		MQTTPassword:    env("MQTT_PASSWORD", ""),
		MQTTCACertPath:  env("MQTT_CA_CERT_PATH", ""),
		MQTTCertPath:    env("MQTT_CERT_PATH", ""),
		MQTTKeyPath:     env("MQTT_KEY_PATH", ""),
		MQTTInsecureSkipVerify: envBool("MQTT_INSECURE_SKIP_VERIFY", false),
		MQTTKeepAlive:   envDuration("MQTT_KEEP_ALIVE", 30*time.Second),
		MQTTSubClientID:   env("MQTT_SUB_CLIENT_ID", ""),
		MQTTSubCACertPath: env("MQTT_SUB_CA_CERT_PATH", ""),
		MQTTSubCertPath:   env("MQTT_SUB_CERT_PATH", ""),
		MQTTSubKeyPath:    env("MQTT_SUB_KEY_PATH", ""),
	}
	if cfg.MQTTSubClientID == "" {
		cfg.MQTTSubClientID = cfg.MQTTClientID
	}
	if cfg.MQTTSubCACertPath == "" {
		cfg.MQTTSubCACertPath = cfg.MQTTCACertPath
	}
	if cfg.MQTTSubCertPath == "" {
		cfg.MQTTSubCertPath = cfg.MQTTCertPath
	}
	if cfg.MQTTSubKeyPath == "" {
		cfg.MQTTSubKeyPath = cfg.MQTTKeyPath
	}

	switch cfg.AuthMode {
	case "local", "cognito":
	default:
		return Config{}, fmt.Errorf("AUTH_MODE must be local or cognito")
	}
	if cfg.AuthMode == "local" && cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required when AUTH_MODE=local")
	}
	if cfg.AuthMode == "cognito" && (cfg.JWTJWKSURL == "" || cfg.JWTIssuer == "") {
		return Config{}, fmt.Errorf("JWT_JWKS_URL and JWT_ISSUER are required when AUTH_MODE=cognito")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
