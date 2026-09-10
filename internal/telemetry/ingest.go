package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
	"github.com/surajkadam7/iot-backend/internal/state"
)

var (
	ErrMalformed     = errors.New("malformed telemetry")
	ErrUnknownDevice = errors.New("unknown device")
	ErrInactive      = errors.New("inactive device")
)

type Broadcaster interface {
	Broadcast(orgID uuid.UUID, reading models.Reading)
}

type Ingestor struct {
	repo  repository.Store
	store *state.Store
	hub   Broadcaster
	log   *slog.Logger
	now   func() time.Time
}

func New(repo repository.Store, store *state.Store, hub Broadcaster, log *slog.Logger) *Ingestor {
	return &Ingestor{
		repo:  repo,
		store: store,
		hub:   hub,
		log:   log,
		now:   time.Now,
	}
}

func (i *Ingestor) HandleMQTT(ctx context.Context, topic string, payload []byte) error {
	identifier := deviceIdentifierFromTopic(topic)
	if identifier == "" {
		i.log.Debug("drop telemetry: bad topic", "topic", topic)
		return ErrMalformed
	}
	reading, err := ParsePayload(payload, i.now())
	if err != nil {
		i.log.Debug("drop telemetry: invalid payload", "device_identifier", identifier, "err", err)
		return ErrMalformed
	}
	device, err := i.repo.GetDeviceByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			i.log.Debug("drop telemetry: unknown device", "device_identifier", identifier)
			return ErrUnknownDevice
		}
		return err
	}
	if device.Status != models.StatusActive {
		i.log.Debug("drop telemetry: inactive device", "device_id", device.ID)
		return ErrInactive
	}
	org, err := i.repo.GetOrganization(ctx, device.OrganizationID)
	if err != nil {
		return err
	}
	if org.Status != models.StatusActive {
		i.log.Debug("drop telemetry: disabled org", "organization_id", org.ID)
		return ErrInactive
	}
	reading.DeviceID = device.ID
	i.store.Set(reading)
	if i.hub != nil {
		i.hub.Broadcast(device.OrganizationID, reading)
	}
	return nil
}

func ParsePayload(raw []byte, receiveTime time.Time) (models.Reading, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return models.Reading{}, ErrMalformed
	}
	temp, err := numberField(m, "temperature")
	if err != nil {
		return models.Reading{}, err
	}
	pressure, err := numberField(m, "pressure")
	if err != nil {
		return models.Reading{}, err
	}
	humidity, err := numberField(m, "humidity")
	if err != nil {
		return models.Reading{}, err
	}
	ts := receiveTime.UTC()
	if rawTS, ok := m["ts"]; ok && rawTS != nil {
		s, ok := rawTS.(string)
		if !ok || s == "" {
			return models.Reading{}, fmt.Errorf("%w: ts", ErrMalformed)
		}
		parsed, err := time.Parse(time.RFC3339, s)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, s)
			if err != nil {
				ts = receiveTime.UTC()
			} else {
				ts = parsed.UTC()
			}
		} else {
			ts = parsed.UTC()
		}
	}
	return models.Reading{
		Temperature: temp,
		Pressure:    pressure,
		Humidity:    humidity,
		TS:          ts,
		UpdatedAt:   receiveTime.UTC(),
	}, nil
}

func numberField(m map[string]any, key string) (float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%w: missing %s", ErrMalformed, key)
	}
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("%w: %s", ErrMalformed, key)
		}
		return f, nil
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, fmt.Errorf("%w: %s", ErrMalformed, key)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%w: %s type", ErrMalformed, key)
	}
}

func deviceIdentifierFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 || parts[0] != "org" || parts[2] != "device" || parts[4] != "telemetry" {
		return ""
	}
	if parts[1] == "" || parts[3] == "" {
		return ""
	}
	return parts[3]
}
