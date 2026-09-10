package iot

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

type Subscriber struct {
	client mqtt.Client
	log    *slog.Logger
}

func NewSubscriber(cfg config.Config, ingest *telemetry.Ingestor, log *slog.Logger) (*Subscriber, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTBroker)
	opts.SetClientID(cfg.MQTTClientID)
	opts.SetKeepAlive(cfg.MQTTKeepAlive)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetOrderMatters(false)
	opts.SetCleanSession(true)
	if cfg.MQTTUsername != "" {
		opts.SetUsername(cfg.MQTTUsername)
		opts.SetPassword(cfg.MQTTPassword)
	}
	tlsCfg, err := tlsConfig(cfg)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	s := &Subscriber{log: log}
	opts.OnConnect = func(c mqtt.Client) {
		log.Info("mqtt connected", "broker", cfg.MQTTBroker)
		tok := c.Subscribe(TelemetryTopicFilter, 1, func(_ mqtt.Client, msg mqtt.Message) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := ingest.HandleMQTT(ctx, msg.Topic(), msg.Payload()); err != nil {
				log.Debug("mqtt message not applied", "err", err)
			}
		})
		if tok.Wait() && tok.Error() != nil {
			log.Error("mqtt subscribe failed", "err", tok.Error())
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Warn("mqtt connection lost", "err", err)
	}
	opts.OnReconnecting = func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		log.Info("mqtt reconnecting")
	}
	s.client = mqtt.NewClient(opts)
	return s, nil
}

func (s *Subscriber) Start(ctx context.Context) error {
	tok := s.client.Connect()
	if !tok.WaitTimeout(15 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := tok.Error(); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	go func() {
		<-ctx.Done()
		s.client.Disconnect(250)
	}()
	return nil
}

func tlsConfig(cfg config.Config) (*tls.Config, error) {
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
