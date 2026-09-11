package telemetry

import (
	"context"
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
func (f *fakeRepo) ListOrganizations(context.Context) ([]models.Organization, error) { return nil, nil }
func (f *fakeRepo) GetUserByID(context.Context, uuid.UUID) (models.User, error) {
	return models.User{}, repository.ErrNotFound
}
func (f *fakeRepo) GetUserBySubject(context.Context, string) (models.User, error) {
	return models.User{}, repository.ErrNotFound
}
func (f *fakeRepo) GetUserByEmail(context.Context, string) (models.User, error) {
	return models.User{}, repository.ErrNotFound
}
func (f *fakeRepo) GetUser(context.Context, uuid.UUID, uuid.UUID) (models.User, error) {
	return models.User{}, repository.ErrNotFound
}
func (f *fakeRepo) ListUsers(context.Context, uuid.UUID) ([]models.User, error) { return nil, nil }
func (f *fakeRepo) ListAllUsers(context.Context) ([]models.User, error)         { return nil, nil }
func (f *fakeRepo) CountUsers(context.Context, uuid.UUID) (int, error)          { return 0, nil }
func (f *fakeRepo) CountActiveAdmins(context.Context, uuid.UUID) (int, error)   { return 0, nil }
func (f *fakeRepo) CreateUser(context.Context, models.User) (models.User, error) {
	return models.User{}, nil
}
func (f *fakeRepo) UpdateUser(context.Context, uuid.UUID, uuid.UUID, *string, *bool, *string) (models.User, error) {
	return models.User{}, nil
}
func (f *fakeRepo) DeleteUser(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeRepo) ListLocations(context.Context, uuid.UUID) ([]models.Location, error) {
	return nil, nil
}
func (f *fakeRepo) ListAllLocations(context.Context) ([]models.Location, error) { return nil, nil }
func (f *fakeRepo) CreateLocation(context.Context, uuid.UUID, string) (models.Location, error) {
	return models.Location{}, nil
}
func (f *fakeRepo) GetLocation(context.Context, uuid.UUID, uuid.UUID) (models.Location, error) {
	return models.Location{}, repository.ErrNotFound
}
func (f *fakeRepo) ListSubLocations(context.Context, uuid.UUID, uuid.UUID) ([]models.SubLocation, error) {
	return nil, nil
}
func (f *fakeRepo) ListAllSubLocations(context.Context) ([]models.SubLocation, error) {
	return nil, nil
}
func (f *fakeRepo) CreateSubLocation(context.Context, uuid.UUID, uuid.UUID, string) (models.SubLocation, error) {
	return models.SubLocation{}, nil
}
func (f *fakeRepo) GetSubLocation(context.Context, uuid.UUID, uuid.UUID) (models.SubLocation, error) {
	return models.SubLocation{}, repository.ErrNotFound
}
func (f *fakeRepo) ListDevices(context.Context, uuid.UUID) ([]models.Device, error) { return nil, nil }
func (f *fakeRepo) ListAllDevices(context.Context) ([]models.Device, error)         { return nil, nil }
func (f *fakeRepo) GetDevice(context.Context, uuid.UUID, uuid.UUID) (models.Device, error) {
	return models.Device{}, repository.ErrNotFound
}
func (f *fakeRepo) GetDeviceByIdentifier(_ context.Context, identifier string) (models.Device, error) {
	d, ok := f.devices[identifier]
	if !ok {
		return models.Device{}, repository.ErrNotFound
	}
	return d, nil
}
func (f *fakeRepo) CreateDevice(context.Context, models.Device) (models.Device, error) {
	return models.Device{}, nil
}
func (f *fakeRepo) UpdateDevice(context.Context, uuid.UUID, uuid.UUID, *string, *string, **uuid.UUID) (models.Device, error) {
	return models.Device{}, nil
}
func (f *fakeRepo) DeleteDevice(context.Context, uuid.UUID, uuid.UUID) error { return nil }

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
