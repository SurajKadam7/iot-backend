package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/observability"
	"github.com/surajkadam7/iot-backend/internal/state"
)

type fixture struct {
	handler http.Handler
	store   *memStore
	v       *auth.Validator
	orgA    uuid.UUID
	orgB    uuid.UUID
	adminA  models.User
	userA   models.User
	adminB  models.User
	devA    models.Device
	devB    models.Device
}

func setupAPI(t *testing.T) *fixture {
	t.Helper()
	orgA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	orgB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	adminA := models.User{
		ID: uuid.MustParse("11111111-1111-4111-8111-111111111101"), OrganizationID: orgA,
		CognitoSubject: "admin-a", Email: "admin-a@example.com", Role: models.RoleOrgAdmin, Status: models.StatusActive,
	}
	userA := models.User{
		ID: uuid.MustParse("11111111-1111-4111-8111-111111111102"), OrganizationID: orgA,
		CognitoSubject: "user-a", Email: "user-a@example.com", Role: models.RoleOrgUser, Status: models.StatusActive,
	}
	adminB := models.User{
		ID: uuid.MustParse("22222222-2222-4222-8222-222222222201"), OrganizationID: orgB,
		CognitoSubject: "admin-b", Email: "admin-b@example.com", Role: models.RoleOrgAdmin, Status: models.StatusActive,
	}
	ms := newMemStore()
	ms.orgs[orgA] = models.Organization{ID: orgA, Name: "Org A", Status: models.StatusActive}
	ms.orgs[orgB] = models.Organization{ID: orgB, Name: "Org B", Status: models.StatusActive}
	ms.users[adminA.ID] = adminA
	ms.users[userA.ID] = userA
	ms.users[adminB.ID] = adminB
	devA := models.Device{ID: uuid.New(), OrganizationID: orgA, DeviceIdentifier: "dev-a", Name: "Device A", Status: models.StatusActive}
	devB := models.Device{ID: uuid.New(), OrganizationID: orgB, DeviceIdentifier: "dev-b", Name: "Device B", Status: models.StatusActive}
	ms.devices[devA.ID] = devA
	ms.devices[devB.ID] = devB

	v, err := auth.NewValidator("local", "test-secret-please-change", "iot-local", "", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AuthMode: "local", JWTSecret: "test-secret-please-change", JWTIssuer: "iot-local", JWTExpiry: time.Hour, FrontendOrigin: "*"}
	h := New(cfg, observability.NewLogger("error", "text"), nil, ms, state.New(), v, nil).Handler()
	return &fixture{handler: h, store: ms, v: v, orgA: orgA, orgB: orgB, adminA: adminA, userA: userA, adminB: adminB, devA: devA, devB: devB}
}

func (f *fixture) token(t *testing.T, u models.User) string {
	t.Helper()
	tok, err := f.v.MintLocal(u, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (f *fixture) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func TestHealthUnauthenticated(t *testing.T) {
	f := setupAPI(t)
	w := f.do(t, http.MethodGet, "/api/health", "", nil)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestDevicesTenantIsolation(t *testing.T) {
	f := setupAPI(t)
	tokA := f.token(t, f.adminA)
	w := f.do(t, http.MethodGet, "/api/devices?organization_id="+f.orgB.String(), tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(resp.Devices))
	}
	if resp.Devices[0]["device_identifier"] != "dev-a" {
		t.Fatalf("leaked foreign tenant: %#v", resp.Devices)
	}

	w = f.do(t, http.MethodGet, "/api/devices/"+f.devB.ID.String(), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign device, got %d", w.Code)
	}
}

func TestOrgUserCannotMutateDevices(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.userA)
	w := f.do(t, http.MethodPost, "/api/devices", tok, map[string]any{
		"device_identifier": "new-dev",
		"name":              "Nope",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPatch, "/api/devices/"+f.devA.ID.String(), tok, map[string]any{"name": "x"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 patch, got %d", w.Code)
	}
	w = f.do(t, http.MethodDelete, "/api/devices/"+f.devA.ID.String(), tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 delete, got %d", w.Code)
	}
}

func TestOrgAdminCreatesDevice(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.adminA)
	w := f.do(t, http.MethodPost, "/api/devices", tok, map[string]any{
		"device_identifier": "line-c-01",
		"name":              "Line C",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestMissingAuth(t *testing.T) {
	f := setupAPI(t)
	w := f.do(t, http.MethodGet, "/api/me", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMeUsesJWTOrgNotBody(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.adminA)
	w := f.do(t, http.MethodGet, "/api/me", tok, nil)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var me map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me["organization_id"] != f.orgA.String() {
		t.Fatalf("org %v", me["organization_id"])
	}
}
