package telemetry

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Record is a validated telemetry event after device-table authorization.
// Archive (and a later Firehose adapter) consume this; they must not re-parse MQTT.
type Record struct {
	OrganizationID   uuid.UUID
	DeviceID         uuid.UUID
	DeviceIdentifier string
	Temperature      float64
	Pressure         float64
	Humidity         float64
	TS               time.Time
}

// Sink is a post-authorization handler. Live memory stays on the ingest hot path;
// extra sinks (CSV archive today, Firehose later) plug in without changing MQTT.
type Sink interface {
	Name() string
	HandleRecord(ctx context.Context, rec Record) error
}
