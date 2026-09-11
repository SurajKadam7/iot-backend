package api

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
)

type memStore struct {
	mu      sync.Mutex
	orgs    map[uuid.UUID]models.Organization
	users   map[uuid.UUID]models.User
	locs    map[uuid.UUID]models.Location
	subs    map[uuid.UUID]models.SubLocation
	devices map[uuid.UUID]models.Device
}

func newMemStore() *memStore {
	return &memStore{
		orgs:    map[uuid.UUID]models.Organization{},
		users:   map[uuid.UUID]models.User{},
		locs:    map[uuid.UUID]models.Location{},
		subs:    map[uuid.UUID]models.SubLocation{},
		devices: map[uuid.UUID]models.Device{},
	}
}

func (m *memStore) GetOrganization(_ context.Context, id uuid.UUID) (models.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[id]
	if !ok {
		return models.Organization{}, repository.ErrNotFound
	}
	return o, nil
}

func (m *memStore) ListOrganizations(_ context.Context) ([]models.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Organization
	for _, o := range m.orgs {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if out == nil {
		out = []models.Organization{}
	}
	return out, nil
}

func (m *memStore) GetUserByID(_ context.Context, id uuid.UUID) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return models.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (m *memStore) GetUserBySubject(_ context.Context, subject string) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.CognitoSubject == subject {
			return u, nil
		}
	}
	return models.User{}, repository.ErrNotFound
}

func (m *memStore) GetUserByEmail(_ context.Context, email string) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return models.User{}, repository.ErrNotFound
}

func (m *memStore) GetUser(_ context.Context, orgID, id uuid.UUID) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok || u.OrganizationID != orgID {
		return models.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (m *memStore) ListUsers(_ context.Context, orgID uuid.UUID) ([]models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.User
	for _, u := range m.users {
		if u.OrganizationID == orgID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	if out == nil {
		out = []models.User{}
	}
	return out, nil
}

func (m *memStore) ListAllUsers(_ context.Context) ([]models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.User
	for _, u := range m.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	if out == nil {
		out = []models.User{}
	}
	return out, nil
}

func (m *memStore) CountUsers(_ context.Context, orgID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, u := range m.users {
		if u.OrganizationID == orgID {
			n++
		}
	}
	return n, nil
}

func (m *memStore) CountActiveAdmins(_ context.Context, orgID uuid.UUID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, u := range m.users {
		if u.OrganizationID == orgID && u.Role == models.RoleOrgAdmin && u.Status == models.StatusActive {
			n++
		}
	}
	return n, nil
}

func (m *memStore) CreateUser(_ context.Context, u models.User) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.users {
		if strings.EqualFold(existing.Email, u.Email) || existing.CognitoSubject == u.CognitoSubject {
			return models.User{}, repository.ErrConflict
		}
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.Status == "" {
		u.Status = models.StatusActive
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *memStore) UpdateUser(_ context.Context, orgID, id uuid.UUID, role *string, canExport *bool, status *string) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok || u.OrganizationID != orgID {
		return models.User{}, repository.ErrNotFound
	}
	if role != nil {
		u.Role = *role
	}
	if canExport != nil {
		u.CanExport = *canExport
	}
	if status != nil {
		u.Status = *status
	}
	m.users[id] = u
	return u, nil
}

func (m *memStore) DeleteUser(_ context.Context, orgID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok || u.OrganizationID != orgID {
		return repository.ErrNotFound
	}
	delete(m.users, id)
	return nil
}

func (m *memStore) ListLocations(_ context.Context, orgID uuid.UUID) ([]models.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Location
	for _, l := range m.locs {
		if l.OrganizationID == orgID {
			out = append(out, l)
		}
	}
	if out == nil {
		out = []models.Location{}
	}
	return out, nil
}

func (m *memStore) ListAllLocations(_ context.Context) ([]models.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Location
	for _, l := range m.locs {
		out = append(out, l)
	}
	if out == nil {
		out = []models.Location{}
	}
	return out, nil
}

func (m *memStore) CreateLocation(_ context.Context, orgID uuid.UUID, name string) (models.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := models.Location{ID: uuid.New(), OrganizationID: orgID, Name: name}
	m.locs[l.ID] = l
	return l, nil
}

func (m *memStore) GetLocation(_ context.Context, orgID, id uuid.UUID) (models.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.locs[id]
	if !ok || l.OrganizationID != orgID {
		return models.Location{}, repository.ErrNotFound
	}
	return l, nil
}

func (m *memStore) ListSubLocations(_ context.Context, orgID, locationID uuid.UUID) ([]models.SubLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.SubLocation
	for _, s := range m.subs {
		if s.OrganizationID == orgID && s.LocationID == locationID {
			out = append(out, s)
		}
	}
	if out == nil {
		out = []models.SubLocation{}
	}
	return out, nil
}

func (m *memStore) ListAllSubLocations(_ context.Context) ([]models.SubLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.SubLocation
	for _, s := range m.subs {
		out = append(out, s)
	}
	if out == nil {
		out = []models.SubLocation{}
	}
	return out, nil
}

func (m *memStore) CreateSubLocation(_ context.Context, orgID, locationID uuid.UUID, name string) (models.SubLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := models.SubLocation{ID: uuid.New(), OrganizationID: orgID, LocationID: locationID, Name: name}
	m.subs[s.ID] = s
	return s, nil
}

func (m *memStore) GetSubLocation(_ context.Context, orgID, id uuid.UUID) (models.SubLocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[id]
	if !ok || s.OrganizationID != orgID {
		return models.SubLocation{}, repository.ErrNotFound
	}
	return s, nil
}

func (m *memStore) ListDevices(_ context.Context, orgID uuid.UUID) ([]models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Device
	for _, d := range m.devices {
		if d.OrganizationID == orgID {
			out = append(out, d)
		}
	}
	if out == nil {
		out = []models.Device{}
	}
	return out, nil
}

func (m *memStore) ListAllDevices(_ context.Context) ([]models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []models.Device
	for _, d := range m.devices {
		out = append(out, d)
	}
	if out == nil {
		out = []models.Device{}
	}
	return out, nil
}

func (m *memStore) GetDevice(_ context.Context, orgID, id uuid.UUID) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[id]
	if !ok || d.OrganizationID != orgID {
		return models.Device{}, repository.ErrNotFound
	}
	return d, nil
}

func (m *memStore) GetDeviceByIdentifier(_ context.Context, identifier string) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if d.DeviceIdentifier == identifier {
			return d, nil
		}
	}
	return models.Device{}, repository.ErrNotFound
}

func (m *memStore) CreateDevice(_ context.Context, d models.Device) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.devices {
		if existing.DeviceIdentifier == d.DeviceIdentifier {
			return models.Device{}, repository.ErrConflict
		}
	}
	d.ID = uuid.New()
	m.devices[d.ID] = d
	return d, nil
}

func (m *memStore) UpdateDevice(_ context.Context, orgID, id uuid.UUID, name *string, status *string, subLocationID **uuid.UUID) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[id]
	if !ok || d.OrganizationID != orgID {
		return models.Device{}, repository.ErrNotFound
	}
	if name != nil {
		d.Name = *name
	}
	if status != nil {
		d.Status = *status
	}
	if subLocationID != nil {
		d.SubLocationID = *subLocationID
	}
	m.devices[id] = d
	return d, nil
}

func (m *memStore) DeleteDevice(_ context.Context, orgID, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[id]
	if !ok || d.OrganizationID != orgID {
		return repository.ErrNotFound
	}
	delete(m.devices, id)
	return nil
}
