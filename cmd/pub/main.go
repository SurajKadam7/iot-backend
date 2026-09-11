package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/iot"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "pub: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	orgID := env("PUB_ORG_ID", "00000000-0000-4000-8000-000000000001")
	devices := []string{"line-a-01"}
	if v := os.Getenv("PUB_DEVICES"); v != "" {
		devices = splitCSV(v)
	}

	interval := 10 * time.Second
	if v := os.Getenv("PUB_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("PUB_INTERVAL: %w", err)
		}
		if d <= 0 {
			return fmt.Errorf("PUB_INTERVAL must be > 0")
		}
		interval = d
	}

	clientID := env("MQTT_PUB_CLIENT_ID", "iot-dev-publisher")
	pubCfg := cfg
	if v := env("MQTT_PUB_CA_CERT_PATH", ""); v != "" {
		pubCfg.MQTTCACertPath = v
	}
	if v := env("MQTT_PUB_CERT_PATH", ""); v != "" {
		pubCfg.MQTTCertPath = v
	}
	if v := env("MQTT_PUB_KEY_PATH", ""); v != "" {
		pubCfg.MQTTKeyPath = v
	}
	if strings.HasPrefix(pubCfg.MQTTBroker, "ssl://") && (pubCfg.MQTTCertPath == "" || pubCfg.MQTTKeyPath == "") {
		return fmt.Errorf("MQTT_PUB_CERT_PATH/MQTT_CERT_PATH and key path are required when MQTT_BROKER uses ssl://")
	}

	opts := mqtt.NewClientOptions().AddBroker(pubCfg.MQTTBroker).SetClientID(clientID)
	opts.SetKeepAlive(pubCfg.MQTTKeepAlive)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(false)

	var lostMu sync.Mutex
	var lastLost error
	lostAt := time.Time{}
	opts.OnConnectionLost = func(_ mqtt.Client, lost error) {
		lostMu.Lock()
		lastLost = lost
		lostAt = time.Now()
		lostMu.Unlock()
		fmt.Fprintf(os.Stderr, "mqtt connection lost: %v\n", lost)
	}
	tlsCfg, err := iot.TLSConfig(pubCfg)
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	fmt.Printf("mqtt publisher connecting broker=%s client_id=%s tls=%v interval=%s org=%s devices=%s\n",
		cfg.MQTTBroker, clientID, tlsCfg != nil, interval, orgID, strings.Join(devices, ","))

	c := mqtt.NewClient(opts)
	tok := c.Connect()
	if !tok.WaitTimeout(15 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt connect: %w (client id and IoT policy must allow iot:Connect)", tok.Error())
	}
	defer c.Disconnect(250)

	fmt.Printf("mqtt publisher connected; topics org/%s/device/{id}/telemetry\n", orgID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	base := make([][3]float64, len(devices))
	for i := range devices {
		base[i] = [3]float64{21 + rand.Float64()*4, 1010 + rand.Float64()*8, 40 + rand.Float64()*15}
	}
	n := 0
	for {
		select {
		case <-stop:
			return nil
		case t := <-tick.C:
			for i, id := range devices {
				temp := base[i][0] + 0.6*math.Sin(float64(n+i)/6)
				pressure := base[i][1] + 0.4*math.Sin(float64(n+i)/9)
				humidity := base[i][2] + 1.2*math.Sin(float64(n+i)/7)
				body, _ := json.Marshal(map[string]any{
					"temperature": round(temp),
					"pressure":    round(pressure),
					"humidity":    round(humidity),
					"ts":          t.UTC().Format(time.RFC3339),
				})
				topic := iot.TelemetryTopic(orgID, id)
				if !c.IsConnected() {
					fmt.Fprintf(os.Stderr, "publish failed device=%s topic=%s err=not connected\n", id, topic)
					continue
				}
				sentAt := time.Now()
				token := c.Publish(topic, 1, false, body)
				if !token.WaitTimeout(10 * time.Second) {
					fmt.Fprintf(os.Stderr, "publish failed device=%s topic=%s err=timeout waiting for puback\n", id, topic)
					continue
				}
				if err := token.Error(); err != nil {
					fmt.Fprintf(os.Stderr, "publish failed device=%s topic=%s err=%v\n", id, topic, err)
					continue
				}
				// AWS IoT closes the socket on unauthorized publish; PUBACK can still race ahead.
				time.Sleep(400 * time.Millisecond)
				lostMu.Lock()
				lost := lastLost
				when := lostAt
				lostMu.Unlock()
				if !when.IsZero() && !when.Before(sentAt) {
					fmt.Fprintf(os.Stderr, "publish failed device=%s topic=%s err=connection closed after publish (%v); check IoT policy\n", id, topic, lost)
					continue
				}
				if !c.IsConnected() {
					fmt.Fprintf(os.Stderr, "publish failed device=%s topic=%s err=disconnected after publish; check IoT policy\n", id, topic)
					continue
				}
				fmt.Printf("publish ok device=%s topic=%s bytes=%d\n", id, topic, len(body))
			}
			n++
		}
	}
}

func round(v float64) float64 {
	return math.Round(v*10) / 10
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
