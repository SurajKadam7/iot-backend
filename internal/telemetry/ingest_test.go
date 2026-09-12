package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
	"github.com/surajkadam7/iot-backend/internal/state"
)

type fakeRepo struct {
	devices map[string]models.Device
	orgs    map[uuid.UUID]models.Organization
}

func (f *fakeRepo) GetOrganization(_ context.Context, id uuid.UUID) (models.Organization, error) {
	o, ok := f.orgs[id]
	if !ok {
		return models.Organization{}, repository.ErrNotFound
	}
	return o, nil
}

func (f *fakeRepo) GetDeviceByIdentifier(_ context.Context, identifier string) (models.Device, error) {
	d, ok := f.devices[identifier]
	if !ok {
		return models.Device{}, repository.ErrNotFound
	}
	return d, nil
}

var _ DeviceLookup = (*fakeRepo)(nil)

type sink struct{ n int }

func (s *sink) Broadcast(uuid.UUID, models.Reading) { s.n++ }

func TestParsePayloadRequiresNumbers(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if _, err := ParsePayload([]byte(`{"temperature":"1","pressure":1,"humidity":1}`), now); err == nil {
		t.Fatal("string temperature should fail")
	}
	r, err := ParsePayload([]byte(`{"temperature":21.5,"pressure":1013,"humidity":40,"ts":"2026-09-10T00:00:00Z"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Temperature != 21.5 || r.Pressure != 1013 || r.Humidity != 40 {
		t.Fatalf("%v", r)
	}
}

func TestIngestUnknownAndMalformed(t *testing.T) {
	orgID := uuid.New()
	devID := uuid.New()
	repo := &fakeRepo{
		orgs: map[uuid.UUID]models.Organization{orgID: {ID: orgID, Status: models.StatusActive}},
		devices: map[string]models.Device{
			"known": {ID: devID, OrganizationID: orgID, DeviceIdentifier: "known", Status: models.StatusActive},
		},
	}
	st := state.New()
	s := &sink{}
	ing := New(repo, st, s, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	topic := "org/" + orgID.String() + "/device/known/telemetry"
	if err := ing.HandleMQTT(context.Background(), topic, []byte(`{"temperature":1,"pressure":2,"humidity":3}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(devID); !ok {
		t.Fatal("expected store update")
	}
	if s.n != 1 {
		t.Fatalf("broadcast %d", s.n)
	}
	if err := ing.HandleMQTT(context.Background(), topic, []byte(`not-json`)); err != ErrMalformed {
		t.Fatalf("malformed: %v", err)
	}
	if err := ing.HandleMQTT(context.Background(), "org/"+orgID.String()+"/device/unknown/telemetry", []byte(`{"temperature":1,"pressure":2,"humidity":3}`)); err != ErrUnknownDevice {
		t.Fatalf("unknown: %v", err)
	}
	if st.Len() != 1 {
		t.Fatalf("store should stay at 1, got %d", st.Len())
	}
}

func TestOverwriteLatest(t *testing.T) {
	orgID := uuid.New()
	devID := uuid.New()
	repo := &fakeRepo{
		orgs:    map[uuid.UUID]models.Organization{orgID: {ID: orgID, Status: models.StatusActive}},
		devices: map[string]models.Device{"d1": {ID: devID, OrganizationID: orgID, DeviceIdentifier: "d1", Status: models.StatusActive}},
	}
	st := state.New()
	ing := New(repo, st, &sink{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	topic := "org/" + orgID.String() + "/device/d1/telemetry"
	_ = ing.HandleMQTT(context.Background(), topic, []byte(`{"temperature":1,"pressure":2,"humidity":3}`))
	_ = ing.HandleMQTT(context.Background(), topic, []byte(`{"temperature":9,"pressure":2,"humidity":3}`))
	r, _ := st.Get(devID)
	if r.Temperature != 9 {
		t.Fatalf("got %v", r.Temperature)
	}
}

type captureSink struct {
	recs []Record
	err  error
}

func (c *captureSink) Name() string { return "capture" }
func (c *captureSink) HandleRecord(_ context.Context, rec Record) error {
	c.recs = append(c.recs, rec)
	return c.err
}

func TestIngestFansOutAfterAuthorize(t *testing.T) {
	orgID := uuid.New()
	devID := uuid.New()
	repo := &fakeRepo{
		orgs: map[uuid.UUID]models.Organization{orgID: {ID: orgID, Status: models.StatusActive}},
		devices: map[string]models.Device{
			"known": {ID: devID, OrganizationID: orgID, DeviceIdentifier: "known", Status: models.StatusActive},
		},
	}
	st := state.New()
	cap := &captureSink{err: errors.New("sink failed")}
	ing := New(repo, st, &sink{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	ing.AddSink(cap)
	topic := "org/not-the-org/device/known/telemetry"
	if err := ing.HandleMQTT(context.Background(), topic, []byte(`{"temperature":1,"pressure":2,"humidity":3}`)); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(devID); !ok {
		t.Fatal("live path must succeed even if a sink fails")
	}
	if len(cap.recs) != 1 {
		t.Fatalf("sink calls %d", len(cap.recs))
	}
	if cap.recs[0].OrganizationID != orgID || cap.recs[0].DeviceIdentifier != "known" {
		t.Fatalf("authorized record %+v", cap.recs[0])
	}
	if err := ing.HandleMQTT(context.Background(), topic, []byte(`not-json`)); err != ErrMalformed {
		t.Fatalf("malformed: %v", err)
	}
	if len(cap.recs) != 1 {
		t.Fatal("sink must not run for malformed payloads")
	}
}
