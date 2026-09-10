package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
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
	devices := []string{"line-a-01", "line-a-02", "line-b-01", "line-b-02"}
	if v := os.Getenv("PUB_DEVICES"); v != "" {
		devices = splitCSV(v)
	}

	opts := mqtt.NewClientOptions().AddBroker(cfg.MQTTBroker).SetClientID("iot-dev-publisher")
	c := mqtt.NewClient(opts)
	tok := c.Connect()
	if !tok.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if tok.Error() != nil {
		return tok.Error()
	}
	defer c.Disconnect(250)

	fmt.Printf("publishing to %s for org %s\n", cfg.MQTTBroker, orgID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(time.Second)
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
				if token := c.Publish(topic, 1, false, body); token.Wait() && token.Error() != nil {
					fmt.Fprintf(os.Stderr, "publish %s: %v\n", id, token.Error())
				}
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
