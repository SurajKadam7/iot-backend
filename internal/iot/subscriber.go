package iot

import (
	"context"
	"fmt"
	"log/slog"
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
	subCfg := cfg
	subCfg.MQTTClientID = cfg.MQTTSubClientID
	subCfg.MQTTCACertPath = cfg.MQTTSubCACertPath
	subCfg.MQTTCertPath = cfg.MQTTSubCertPath
	subCfg.MQTTKeyPath = cfg.MQTTSubKeyPath

	opts := mqtt.NewClientOptions()
	opts.AddBroker(subCfg.MQTTBroker)
	opts.SetClientID(subCfg.MQTTClientID)
	opts.SetKeepAlive(subCfg.MQTTKeepAlive)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetOrderMatters(false)
	opts.SetCleanSession(true)
	if subCfg.MQTTUsername != "" {
		opts.SetUsername(subCfg.MQTTUsername)
		opts.SetPassword(subCfg.MQTTPassword)
	}
	tlsCfg, err := TLSConfig(subCfg)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	s := &Subscriber{log: log}
	opts.OnConnect = func(c mqtt.Client) {
		log.Info("mqtt connected", "broker", subCfg.MQTTBroker, "client_id", subCfg.MQTTClientID)
		tok := c.Subscribe(TelemetryTopicFilter, 1, func(_ mqtt.Client, msg mqtt.Message) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			payload := string(msg.Payload())
			if err := ingest.HandleMQTT(ctx, msg.Topic(), msg.Payload()); err != nil {
				log.Warn("mqtt received", "topic", msg.Topic(), "payload", payload, "applied", false, "err", err)
				return
			}
			log.Info("mqtt received", "topic", msg.Topic(), "payload", payload, "applied", true)
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
