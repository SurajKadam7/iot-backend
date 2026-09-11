package iot

import (
	"testing"

	"github.com/surajkadam7/iot-backend/internal/config"
)

func TestTLSConfigNone(t *testing.T) {
	cfg, err := TLSConfig(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("expected no TLS for local broker")
	}
}

func TestTLSConfigMissingCA(t *testing.T) {
	_, err := TLSConfig(config.Config{MQTTCACertPath: "missing-ca.pem"})
	if err == nil {
		t.Fatal("expected error")
	}
}
