package iot

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/surajkadam7/iot-backend/internal/config"
)

// TLSConfig returns nil when MQTT is plain TCP (local Mosquitto).
// For AWS IoT Core, set MQTT_CA_CERT_PATH plus MQTT_CERT_PATH / MQTT_KEY_PATH.
func TLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.MQTTCertPath == "" && cfg.MQTTCACertPath == "" && !cfg.MQTTInsecureSkipVerify {
		return nil, nil
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.MQTTInsecureSkipVerify}
	if cfg.MQTTCACertPath != "" {
		pem, err := os.ReadFile(cfg.MQTTCACertPath)
		if err != nil {
			return nil, fmt.Errorf("mqtt ca cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("mqtt ca cert: no PEM")
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.MQTTCertPath != "" || cfg.MQTTKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.MQTTCertPath, cfg.MQTTKeyPath)
		if err != nil {
			return nil, fmt.Errorf("mqtt client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}
