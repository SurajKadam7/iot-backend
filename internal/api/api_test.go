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
	ops     models.User
	devA    models.Device
	devB    models.Device
}

func setupAPI(t *testing.T) *fixture {
	t.Helper()
	orgA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	orgB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	platformOrg := uuid.MustParse("99999999-9999-4999-8999-999999999999")
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
	ops := models.User{
		ID: uuid.MustParse("99999999-9999-4999-8999-999999999901"), OrganizationID: platformOrg,
		CognitoSubject: "ops", Email: "ops@example.com", Role: models.RolePlatformAdmin, Status: models.StatusActive,
	}
	ms := newMemStore()
	ms.orgs[orgA] = models.Organization{ID: orgA, Name: "Org A", Status: models.StatusActive, UserLimit: 25}
	ms.orgs[orgB] = models.Organization{ID: orgB, Name: "Org B", Status: models.StatusActive, UserLimit: 25}
	ms.orgs[platformOrg] = models.Organization{ID: platformOrg, Name: "Internal operators", Status: models.StatusActive, UserLimit: 5}
	ms.users[adminA.ID] = adminA
	ms.users[userA.ID] = userA
	ms.users[adminB.ID] = adminB
	ms.users[ops.ID] = ops
	locA := models.Location{ID: uuid.MustParse("11111111-1111-4111-8111-111111111201"), OrganizationID: orgA, Name: "Plant"}
	subA := models.SubLocation{ID: uuid.MustParse("11111111-1111-4111-8111-111111111202"), OrganizationID: orgA, LocationID: locA.ID, Name: "Line A"}
	ms.locs[locA.ID] = locA
	ms.subs[subA.ID] = subA
	subID := subA.ID
	devA := models.Device{ID: uuid.New(), OrganizationID: orgA, DeviceIdentifier: "dev-a", Name: "Device A", Status: models.StatusActive, SubLocationID: &subID}
	devB := models.Device{ID: uuid.New(), OrganizationID: orgB, DeviceIdentifier: "dev-b", Name: "Device B", Status: models.StatusActive}
	ms.devices[devA.ID] = devA
	ms.devices[devB.ID] = devB

	v, err := auth.NewValidator("local", "test-secret-please-change", "iot-local", "", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AuthMode: "local", JWTSecret: "test-secret-please-change", JWTIssuer: "iot-local", JWTExpiry: time.Hour, FrontendOrigin: "*"}
	h := New(cfg, observability.NewLogger("error", "text"), nil, ms, state.New(), v, nil).Handler()
	return &fixture{handler: h, store: ms, v: v, orgA: orgA, orgB: orgB, adminA: adminA, userA: userA, adminB: adminB, ops: ops, devA: devA, devB: devB}
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

func TestOrgUserCannotManageUsers(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.userA)
	w := f.do(t, http.MethodGet, "/api/users", tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 list, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPost, "/api/users", tok, map[string]any{
		"email": "new@example.com", "role": "org_user", "can_export": false,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 create, got %d", w.Code)
	}
}

func TestUsersTenantIsolation(t *testing.T) {
	f := setupAPI(t)
	tokA := f.token(t, f.adminA)
	w := f.do(t, http.MethodGet, "/api/users", tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Users))
	}
	w = f.do(t, http.MethodDelete, "/api/users/"+f.adminB.ID.String(), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign user, got %d %s", w.Code, w.Body.String())
	}
}

func TestOrgAdminCreatesAndRemovesUser(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.adminA)
	w := f.do(t, http.MethodPost, "/api/users", tok, map[string]any{
		"email": "viewer2@example.com", "role": "org_user", "can_export": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)
	if created["can_export"] != true || created["role"] != "org_user" {
		t.Fatalf("created %#v", created)
	}
	w = f.do(t, http.MethodPost, "/api/users", tok, map[string]any{
		"email": "VIEWER2@example.com", "role": "org_user", "can_export": false,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 duplicate, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPatch, "/api/users/"+id, tok, map[string]any{"can_export": false})
	if w.Code != 200 {
		t.Fatalf("patch %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodDelete, "/api/users/"+id, tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
}

func TestCannotRemoveSelfOrLastAdmin(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.adminA)
	w := f.do(t, http.MethodDelete, "/api/users/"+f.adminA.ID.String(), tok, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 last admin, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPatch, "/api/users/"+f.adminA.ID.String(), tok, map[string]any{"role": "org_user"})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 demote last admin, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPost, "/api/users", tok, map[string]any{
		"email": "admin2@example.com", "role": "org_admin", "can_export": false,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create admin %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	w = f.do(t, http.MethodDelete, "/api/users/"+f.adminA.ID.String(), tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 self delete, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodDelete, "/api/users/"+created["id"].(string), tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete other admin %d %s", w.Code, w.Body.String())
	}
}

func TestUserLimit(t *testing.T) {
	f := setupAPI(t)
	org := f.store.orgs[f.orgA]
	org.UserLimit = 2
	f.store.orgs[f.orgA] = org
	tok := f.token(t, f.adminA)
	w := f.do(t, http.MethodPost, "/api/users", tok, map[string]any{
		"email": "overflow@example.com", "role": "org_user", "can_export": false,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 user_limit, got %d %s", w.Code, w.Body.String())
	}
}

func TestPermissionOverlayFromDB(t *testing.T) {
	f := setupAPI(t)
	tok := f.token(t, f.userA)
	w := f.do(t, http.MethodPost, "/api/devices", tok, map[string]any{
		"device_identifier": "blocked", "name": "Blocked",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 before promote, got %d", w.Code)
	}
	user := f.store.users[f.userA.ID]
	user.Role = models.RoleOrgAdmin
	f.store.users[f.userA.ID] = user
	w = f.do(t, http.MethodPost, "/api/devices", tok, map[string]any{
		"device_identifier": "after-promote", "name": "After",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 after DB promote, got %d %s", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodGet, "/api/me", tok, nil)
	if w.Code != 200 {
		t.Fatalf("me %d", w.Code)
	}
	var me map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me["role"] != "org_admin" {
		t.Fatalf("me role %v", me["role"])
	}
}

func TestInternalOrganizationsForbiddenToOrgRoles(t *testing.T) {
	f := setupAPI(t)
	for _, u := range []models.User{f.adminA, f.userA} {
		w := f.do(t, http.MethodGet, "/api/internal/organizations", f.token(t, u), nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s expected 403, got %d %s", u.Email, w.Code, w.Body.String())
		}
	}
}

func TestPlatformAdminListsAllOrganizations(t *testing.T) {
	f := setupAPI(t)
	w := f.do(t, http.MethodGet, "/api/internal/organizations", f.token(t, f.ops), nil)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Organizations []struct {
			Name                  string `json:"name"`
			UserCount             int    `json:"user_count"`
			DeviceCount           int    `json:"device_count"`
			UnassignedDeviceCount int    `json:"unassigned_device_count"`
			Users                 []struct {
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"users"`
			Locations []struct {
				Name         string `json:"name"`
				DeviceCount  int    `json:"device_count"`
				SubLocations []struct {
					Name        string `json:"name"`
					DeviceCount int    `json:"device_count"`
				} `json:"sub_locations"`
			} `json:"locations"`
		} `json:"organizations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Organizations) != 3 {
		t.Fatalf("expected 3 orgs, got %d", len(resp.Organizations))
	}
	byName := map[string]int{}
	for i, o := range resp.Organizations {
		byName[o.Name] = i
	}
	a := resp.Organizations[byName["Org A"]]
	if a.UserCount != 2 || a.DeviceCount != 1 {
		t.Fatalf("org A counts users=%d devices=%d", a.UserCount, a.DeviceCount)
	}
	emails := map[string]string{}
	for _, u := range a.Users {
		emails[u.Email] = u.Role
	}
	if emails["admin-a@example.com"] != models.RoleOrgAdmin || emails["user-a@example.com"] != models.RoleOrgUser {
		t.Fatalf("org A users %#v", a.Users)
	}
	if len(a.Locations) != 1 || a.Locations[0].Name != "Plant" || a.Locations[0].DeviceCount != 1 {
		t.Fatalf("org A locations %#v", a.Locations)
	}
	if len(a.Locations[0].SubLocations) != 1 || a.Locations[0].SubLocations[0].DeviceCount != 1 {
		t.Fatalf("org A sub %#v", a.Locations[0].SubLocations)
	}
	b := resp.Organizations[byName["Org B"]]
	if b.DeviceCount != 1 || b.UnassignedDeviceCount != 1 {
		t.Fatalf("org B devices=%d unassigned=%d", b.DeviceCount, b.UnassignedDeviceCount)
	}
}

func TestCannotCreatePlatformAdminViaOrgAPI(t *testing.T) {
	f := setupAPI(t)
	w := f.do(t, http.MethodPost, "/api/users", f.token(t, f.adminA), map[string]any{
		"email": "evil@example.com", "role": "platform_admin", "can_export": false,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", w.Code, w.Body.String())
	}
}
