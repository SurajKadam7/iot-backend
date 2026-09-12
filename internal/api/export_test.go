package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/surajkadam7/iot-backend/internal/archive"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

func TestExportRequiresCanExport(t *testing.T) {
	f := setupAPI(t)
	body := map[string]any{
		"from": "2026-04-01T00:00:00Z",
		"to":   "2026-04-01T23:59:59Z",
	}
	w := f.do(t, http.MethodPost, "/api/exports", f.token(t, f.adminA), body)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin without can_export expected 403, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPost, "/api/exports", f.token(t, f.userA), body)
	if w.Code != http.StatusForbidden {
		t.Fatalf("user without can_export expected 403, got %d %s", w.Code, w.Body.String())
	}
}

func TestExportJobTenantIsolation(t *testing.T) {
	f := setupAPI(t)
	exporter := f.adminA
	exporter.CanExport = true
	f.store.users[exporter.ID] = exporter
	other := f.adminB
	other.CanExport = true
	f.store.users[other.ID] = other

	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	putArchive(t, f, telemetry.Record{
		OrganizationID: f.orgA, DeviceIdentifier: "dev-a", Temperature: 1.5, Pressure: 1000, Humidity: 30, TS: ts,
	})
	putArchive(t, f, telemetry.Record{
		OrganizationID: f.orgB, DeviceIdentifier: "dev-b", Temperature: 99, Pressure: 99, Humidity: 99, TS: ts,
	})

	w := f.do(t, http.MethodPost, "/api/exports", f.token(t, exporter), map[string]any{
		"from": "2026-04-01T00:00:00Z",
		"to":   "2026-04-01T23:59:59Z",
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)
	if _, err := f.server.ProcessExportJobs(context.Background()); err != nil {
		t.Fatal(err)
	}
	w = f.do(t, http.MethodGet, "/api/exports/"+id, f.token(t, other), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign job expected 404, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodGet, "/api/exports/"+id, f.token(t, exporter), nil)
	if w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	var job map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if job["status"] != models.ExportSucceeded {
		t.Fatalf("status %#v", job)
	}
	if job["download_url"] != "/api/exports/"+id+"/file" {
		t.Fatalf("download_url %#v", job["download_url"])
	}
	w = f.do(t, http.MethodGet, "/api/exports/"+id+"/file", f.token(t, exporter), nil)
	if w.Code != 200 {
		t.Fatalf("file %d %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("content-type %s", ct)
	}
	rows := readCSV(t, w.Body.Bytes())
	if len(rows) != 2 {
		t.Fatalf("want header+1 row, got %d %#v", len(rows), rows)
	}
	if !archive.IsHeader(rows[0]) {
		t.Fatalf("header %#v", rows[0])
	}
	if rows[1][0] != f.orgA.String() || rows[1][1] != "dev-a" {
		t.Fatalf("row %#v", rows[1])
	}
	for _, row := range rows[1:] {
		if row[1] == "dev-b" || row[0] == f.orgB.String() {
			t.Fatalf("leaked org B %#v", row)
		}
	}

	w = f.do(t, http.MethodGet, "/api/exports/"+id+"/file", f.token(t, other), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign file expected 404, got %d", w.Code)
	}
}

func TestExportUnknownDeviceRejected(t *testing.T) {
	f := setupAPI(t)
	exporter := f.adminA
	exporter.CanExport = true
	f.store.users[exporter.ID] = exporter
	w := f.do(t, http.MethodPost, "/api/exports", f.token(t, exporter), map[string]any{
		"from":       "2026-04-01T00:00:00Z",
		"to":         "2026-04-01T01:00:00Z",
		"device_ids": []string{"dev-b"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for foreign device, got %d %s", w.Code, w.Body.String())
	}
}

func putArchive(t *testing.T, f *fixture, rec telemetry.Record) {
	t.Helper()
	w := archive.NewWriter(f.objects, nil, archive.WriterOptions{FlushRows: 1, NewID: func() string { return rec.DeviceIdentifier }})
	if err := w.HandleRecord(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
}

func readCSV(t *testing.T, raw []byte) [][]string {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(raw))
	rows, err := r.ReadAll()
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		t.Fatal("empty csv")
	}
	return rows
}
