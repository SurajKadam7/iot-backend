package archive

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/storage"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

func TestCSVRoundTrip(t *testing.T) {
	org := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	rec := telemetry.Record{
		OrganizationID:   org,
		DeviceIdentifier: "line-a-01",
		Temperature:      21.5,
		Pressure:         1013,
		Humidity:         40,
		TS:               time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}
	got, err := DecodeRow(EncodeRow(rec))
	if err != nil {
		t.Fatal(err)
	}
	if got.OrganizationID != rec.OrganizationID || got.DeviceIdentifier != rec.DeviceIdentifier {
		t.Fatalf("%+v", got)
	}
	if got.Temperature != 21.5 || got.Pressure != 1013 || got.Humidity != 40 {
		t.Fatalf("%+v", got)
	}
	if !got.TS.Equal(rec.TS) {
		t.Fatalf("ts %s", got.TS)
	}
}

func TestWriterFlushAndPrefix(t *testing.T) {
	objects := storage.NewMemory()
	w := NewWriter(objects, nil, WriterOptions{FlushRows: 1, NewID: func() string { return "part1" }})
	org := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	ts := time.Date(2026, 4, 1, 15, 30, 0, 0, time.UTC)
	if err := w.HandleRecord(context.Background(), telemetry.Record{
		OrganizationID: org, DeviceIdentifier: "dev-1", Temperature: 1, Pressure: 2, Humidity: 3, TS: ts,
	}); err != nil {
		t.Fatal(err)
	}
	keys, err := objects.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("keys %#v", keys)
	}
	wantPrefix := "org/" + org.String() + "/device/dev-1/date=2026-04-01/hour=15/"
	if !strings.HasPrefix(keys[0], wantPrefix) || !strings.HasSuffix(keys[0], ".csv") {
		t.Fatalf("key %s", keys[0])
	}
	rc, err := objects.Get(context.Background(), keys[0])
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "organization_id,device_identifier,temperature,pressure,humidity,ts") {
		t.Fatalf("header %s", body)
	}
	if !strings.Contains(string(body), org.String()) || !strings.Contains(string(body), "dev-1") {
		t.Fatalf("row %s", body)
	}
}

func TestMergeIsolatesOrganization(t *testing.T) {
	objects := storage.NewMemory()
	orgA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	orgB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ts := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	putCSV(t, objects, telemetry.Record{OrganizationID: orgA, DeviceIdentifier: "dev-a", Temperature: 1, Pressure: 1, Humidity: 1, TS: ts})
	putCSV(t, objects, telemetry.Record{OrganizationID: orgB, DeviceIdentifier: "dev-b", Temperature: 9, Pressure: 9, Humidity: 9, TS: ts})

	q := Query{OrganizationID: orgA, From: ts.Add(-time.Hour), To: ts.Add(time.Hour)}
	keys, err := ListMatching(context.Background(), objects, q)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := MergeCSV(context.Background(), objects, keys, q, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, orgB.String()) || strings.Contains(out, "dev-b") {
		t.Fatalf("leaked org B: %s", out)
	}
	if !strings.Contains(out, orgA.String()) || !strings.Contains(out, "dev-a") {
		t.Fatalf("missing org A: %s", out)
	}
}

func TestDeviceFilter(t *testing.T) {
	objects := storage.NewMemory()
	org := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ts := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	putCSV(t, objects, telemetry.Record{OrganizationID: org, DeviceIdentifier: "dev-a", Temperature: 1, Pressure: 1, Humidity: 1, TS: ts})
	putCSV(t, objects, telemetry.Record{OrganizationID: org, DeviceIdentifier: "dev-b", Temperature: 2, Pressure: 2, Humidity: 2, TS: ts})
	q := Query{OrganizationID: org, DeviceIDs: []string{"dev-a"}, From: ts.Add(-time.Minute), To: ts.Add(time.Minute)}
	keys, err := ListMatching(context.Background(), objects, q)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := MergeCSV(context.Background(), objects, keys, q, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "dev-b") {
		t.Fatalf("leaked device: %s", buf.String())
	}
}

func putCSV(t *testing.T, objects storage.Store, rec telemetry.Record) {
	t.Helper()
	w := NewWriter(objects, nil, WriterOptions{FlushRows: 1, NewID: func() string { return rec.DeviceIdentifier }})
	if err := w.HandleRecord(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
}
