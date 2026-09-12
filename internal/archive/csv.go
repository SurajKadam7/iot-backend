package archive

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

// Header is the stable Phase 1 CSV schema. Archive writes and export reads
// must use this exact order so a later Firehose layout can match.
func Header() []string {
	return []string{"organization_id", "device_identifier", "temperature", "pressure", "humidity", "ts"}
}

func EncodeRow(rec telemetry.Record) []string {
	return []string{
		rec.OrganizationID.String(),
		rec.DeviceIdentifier,
		formatFloat(rec.Temperature),
		formatFloat(rec.Pressure),
		formatFloat(rec.Humidity),
		rec.TS.UTC().Format(time.RFC3339Nano),
	}
}

func DecodeRow(cols []string) (telemetry.Record, error) {
	if len(cols) != 6 {
		return telemetry.Record{}, fmt.Errorf("csv row: want 6 columns, got %d", len(cols))
	}
	orgID, err := uuid.Parse(strings.TrimSpace(cols[0]))
	if err != nil {
		return telemetry.Record{}, fmt.Errorf("csv organization_id: %w", err)
	}
	ident := strings.Clone(strings.TrimSpace(cols[1]))
	if ident == "" {
		return telemetry.Record{}, fmt.Errorf("csv device_identifier is empty")
	}
	temp, err := strconv.ParseFloat(cols[2], 64)
	if err != nil {
		return telemetry.Record{}, fmt.Errorf("csv temperature: %w", err)
	}
	pressure, err := strconv.ParseFloat(cols[3], 64)
	if err != nil {
		return telemetry.Record{}, fmt.Errorf("csv pressure: %w", err)
	}
	humidity, err := strconv.ParseFloat(cols[4], 64)
	if err != nil {
		return telemetry.Record{}, fmt.Errorf("csv humidity: %w", err)
	}
	ts, err := parseTS(cols[5])
	if err != nil {
		return telemetry.Record{}, fmt.Errorf("csv ts: %w", err)
	}
	return telemetry.Record{
		OrganizationID:   orgID,
		DeviceIdentifier: ident,
		Temperature:      temp,
		Pressure:         pressure,
		Humidity:         humidity,
		TS:               ts,
	}, nil
}

func IsHeader(cols []string) bool {
	want := Header()
	if len(cols) != len(want) {
		return false
	}
	for i := range want {
		if strings.TrimSpace(cols[i]) != want[i] {
			return false
		}
	}
	return true
}

func WriteCSV(w io.Writer, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(Header()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func parseTS(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
