package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/observability"
	"github.com/surajkadam7/iot-backend/internal/state"
)

type mem struct {
	users   map[uuid.UUID]models.User
	orgs    map[uuid.UUID]models.Organization
	devices map[uuid.UUID]models.Device
}

func (m *mem) GetUserByID(_ context.Context, id uuid.UUID) (models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return models.User{}, errNF
	}
	return u, nil
}
func (m *mem) GetOrganization(_ context.Context, id uuid.UUID) (models.Organization, error) {
	o, ok := m.orgs[id]
	if !ok {
		return models.Organization{}, errNF
	}
	return o, nil
}
func (m *mem) GetUserBySubject(_ context.Context, sub string) (models.User, error) {
	for _, u := range m.users {
		if u.CognitoSubject == sub {
			return u, nil
		}
	}
	return models.User{}, errNF
}
func (m *mem) GetUserByEmail(context.Context, string) (models.User, error) { return models.User{}, errNF }
func (m *mem) ListLocations(context.Context, uuid.UUID) ([]models.Location, error) {
	return nil, nil
}
func (m *mem) CreateLocation(context.Context, uuid.UUID, string) (models.Location, error) {
	return models.Location{}, nil
}
func (m *mem) GetLocation(context.Context, uuid.UUID, uuid.UUID) (models.Location, error) {
	return models.Location{}, errNF
}
func (m *mem) ListSubLocations(context.Context, uuid.UUID, uuid.UUID) ([]models.SubLocation, error) {
	return nil, nil
}
func (m *mem) CreateSubLocation(context.Context, uuid.UUID, uuid.UUID, string) (models.SubLocation, error) {
	return models.SubLocation{}, nil
}
func (m *mem) GetSubLocation(context.Context, uuid.UUID, uuid.UUID) (models.SubLocation, error) {
	return models.SubLocation{}, errNF
}
func (m *mem) ListDevices(_ context.Context, orgID uuid.UUID) ([]models.Device, error) {
	var out []models.Device
	for _, d := range m.devices {
		if d.OrganizationID == orgID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *mem) GetDevice(context.Context, uuid.UUID, uuid.UUID) (models.Device, error) {
	return models.Device{}, errNF
}
func (m *mem) GetDeviceByIdentifier(context.Context, string) (models.Device, error) {
	return models.Device{}, errNF
}
func (m *mem) CreateDevice(context.Context, models.Device) (models.Device, error) {
	return models.Device{}, nil
}
func (m *mem) UpdateDevice(context.Context, uuid.UUID, uuid.UUID, *string, *string, **uuid.UUID) (models.Device, error) {
	return models.Device{}, nil
}
func (m *mem) DeleteDevice(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type nf string

func (e nf) Error() string { return string(e) }

var errNF = nf("not found")

func TestWSRequiresAuthBeforeTelemetry(t *testing.T) {
	orgA := uuid.New()
	orgB := uuid.New()
	user := models.User{ID: uuid.New(), OrganizationID: orgA, CognitoSubject: "u", Role: models.RoleOrgUser, Status: models.StatusActive}
	devA := models.Device{ID: uuid.New(), OrganizationID: orgA, DeviceIdentifier: "a", Status: models.StatusActive}
	st := state.New()
	repo := &mem{
		users:   map[uuid.UUID]models.User{user.ID: user},
		orgs:    map[uuid.UUID]models.Organization{orgA: {ID: orgA, Status: models.StatusActive}, orgB: {ID: orgB, Status: models.StatusActive}},
		devices: map[uuid.UUID]models.Device{devA.ID: devA},
	}
	v, _ := auth.NewValidator("local", "secret", "iot-local", "", "")
	hub := NewHub(v, repo, st, "*", observability.NewLogger("error", "text"))
	srv := httptest.NewServer(hub)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	st.Set(models.Reading{DeviceID: devA.ID, Temperature: 1})
	hub.Broadcast(orgA, models.Reading{DeviceID: devA.ID, Temperature: 1})

	tok, _ := v.MintLocal(user, time.Hour)
	if err := conn.WriteJSON(map[string]any{"type": "auth", "token": tok}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var first map[string]any
	_ = json.Unmarshal(data, &first)
	if first["type"] != "auth_ok" {
		t.Fatalf("first message after auth must be auth_ok (no pre-auth telemetry), got %#v", first)
	}
	_, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var snap map[string]any
	_ = json.Unmarshal(data, &snap)
	if snap["type"] != "snapshot" {
		t.Fatalf("expected snapshot %#v", snap)
	}

	hub.Broadcast(orgB, models.Reading{DeviceID: uuid.New(), Temperature: 99})
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, msg, err := conn.ReadMessage(); err == nil {
		t.Fatalf("received other org reading: %s", msg)
	}
}

func TestWSAuthTimeout(t *testing.T) {
	repo := &mem{users: map[uuid.UUID]models.User{}, orgs: map[uuid.UUID]models.Organization{}, devices: map[uuid.UUID]models.Device{}}
	v, _ := auth.NewValidator("local", "secret", "iot-local", "", "")
	hub := NewHub(v, repo, state.New(), "*", observability.NewLogger("error", "text"))
	srv := httptest.NewServer(hub)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(6 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected close after timeout")
	}
}
