package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/surajkadam7/iot-backend/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

type Store interface {
	GetOrganization(ctx context.Context, id uuid.UUID) (models.Organization, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserBySubject(ctx context.Context, subject string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)

	ListLocations(ctx context.Context, orgID uuid.UUID) ([]models.Location, error)
	CreateLocation(ctx context.Context, orgID uuid.UUID, name string) (models.Location, error)
	GetLocation(ctx context.Context, orgID, id uuid.UUID) (models.Location, error)

	ListSubLocations(ctx context.Context, orgID, locationID uuid.UUID) ([]models.SubLocation, error)
	CreateSubLocation(ctx context.Context, orgID, locationID uuid.UUID, name string) (models.SubLocation, error)
	GetSubLocation(ctx context.Context, orgID, id uuid.UUID) (models.SubLocation, error)

	ListDevices(ctx context.Context, orgID uuid.UUID) ([]models.Device, error)
	GetDevice(ctx context.Context, orgID, id uuid.UUID) (models.Device, error)
	GetDeviceByIdentifier(ctx context.Context, identifier string) (models.Device, error)
	CreateDevice(ctx context.Context, d models.Device) (models.Device, error)
	UpdateDevice(ctx context.Context, orgID, id uuid.UUID, name *string, status *string, subLocationID **uuid.UUID) (models.Device, error)
	DeleteDevice(ctx context.Context, orgID, id uuid.UUID) error
}

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) GetOrganization(ctx context.Context, id uuid.UUID) (models.Organization, error) {
	var o models.Organization
	err := p.pool.QueryRow(ctx, `
		SELECT id, name, user_limit, status, created_at
		FROM organizations WHERE id=$1`, id).Scan(&o.ID, &o.Name, &o.UserLimit, &o.Status, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Organization{}, ErrNotFound
	}
	return o, err
}

func (p *Postgres) GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	return p.scanUser(p.pool.QueryRow(ctx, userSelect+" WHERE id=$1", id))
}

func (p *Postgres) GetUserBySubject(ctx context.Context, subject string) (models.User, error) {
	return p.scanUser(p.pool.QueryRow(ctx, userSelect+" WHERE cognito_subject=$1", subject))
}

func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	return p.scanUser(p.pool.QueryRow(ctx, userSelect+" WHERE lower(email)=lower($1)", email))
}

const userSelect = `SELECT id, organization_id, cognito_subject, email, role, can_export, status, created_at FROM users`

func (p *Postgres) scanUser(row pgx.Row) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.OrganizationID, &u.CognitoSubject, &u.Email, &u.Role, &u.CanExport, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func (p *Postgres) ListLocations(ctx context.Context, orgID uuid.UUID) ([]models.Location, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, organization_id, name, created_at
		FROM locations WHERE organization_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Location
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if out == nil {
		out = []models.Location{}
	}
	return out, rows.Err()
}

func (p *Postgres) CreateLocation(ctx context.Context, orgID uuid.UUID, name string) (models.Location, error) {
	var l models.Location
	err := p.pool.QueryRow(ctx, `
		INSERT INTO locations (organization_id, name)
		VALUES ($1, $2)
		RETURNING id, organization_id, name, created_at`, orgID, name).
		Scan(&l.ID, &l.OrganizationID, &l.Name, &l.CreatedAt)
	return l, err
}

func (p *Postgres) GetLocation(ctx context.Context, orgID, id uuid.UUID) (models.Location, error) {
	var l models.Location
	err := p.pool.QueryRow(ctx, `
		SELECT id, organization_id, name, created_at
		FROM locations WHERE id=$1 AND organization_id=$2`, id, orgID).
		Scan(&l.ID, &l.OrganizationID, &l.Name, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Location{}, ErrNotFound
	}
	return l, err
}

func (p *Postgres) ListSubLocations(ctx context.Context, orgID, locationID uuid.UUID) ([]models.SubLocation, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, organization_id, location_id, name, created_at
		FROM sub_locations
		WHERE organization_id=$1 AND location_id=$2
		ORDER BY name`, orgID, locationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SubLocation
	for rows.Next() {
		var s models.SubLocation
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.LocationID, &s.Name, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []models.SubLocation{}
	}
	return out, rows.Err()
}

func (p *Postgres) CreateSubLocation(ctx context.Context, orgID, locationID uuid.UUID, name string) (models.SubLocation, error) {
	var s models.SubLocation
	err := p.pool.QueryRow(ctx, `
		INSERT INTO sub_locations (organization_id, location_id, name)
		VALUES ($1, $2, $3)
		RETURNING id, organization_id, location_id, name, created_at`, orgID, locationID, name).
		Scan(&s.ID, &s.OrganizationID, &s.LocationID, &s.Name, &s.CreatedAt)
	return s, err
}

func (p *Postgres) GetSubLocation(ctx context.Context, orgID, id uuid.UUID) (models.SubLocation, error) {
	var s models.SubLocation
	err := p.pool.QueryRow(ctx, `
		SELECT id, organization_id, location_id, name, created_at
		FROM sub_locations WHERE id=$1 AND organization_id=$2`, id, orgID).
		Scan(&s.ID, &s.OrganizationID, &s.LocationID, &s.Name, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.SubLocation{}, ErrNotFound
	}
	return s, err
}

const deviceSelect = `
SELECT d.id, d.organization_id, d.sub_location_id, d.device_identifier, d.name, d.status, d.created_at,
       sl.location_id, loc.name, sl.name
FROM devices d
LEFT JOIN sub_locations sl ON sl.id = d.sub_location_id
LEFT JOIN locations loc ON loc.id = sl.location_id`

func (p *Postgres) ListDevices(ctx context.Context, orgID uuid.UUID) ([]models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelect+` WHERE d.organization_id=$1 ORDER BY d.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []models.Device{}
	}
	return out, rows.Err()
}

func (p *Postgres) GetDevice(ctx context.Context, orgID, id uuid.UUID) (models.Device, error) {
	d, err := scanDevice(p.pool.QueryRow(ctx, deviceSelect+` WHERE d.id=$1 AND d.organization_id=$2`, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Device{}, ErrNotFound
	}
	return d, err
}

func (p *Postgres) GetDeviceByIdentifier(ctx context.Context, identifier string) (models.Device, error) {
	d, err := scanDevice(p.pool.QueryRow(ctx, deviceSelect+` WHERE d.device_identifier=$1`, identifier))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Device{}, ErrNotFound
	}
	return d, err
}

func (p *Postgres) CreateDevice(ctx context.Context, d models.Device) (models.Device, error) {
	var id uuid.UUID
	err := p.pool.QueryRow(ctx, `
		INSERT INTO devices (organization_id, sub_location_id, device_identifier, name, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, d.OrganizationID, d.SubLocationID, d.DeviceIdentifier, d.Name, d.Status).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return models.Device{}, ErrConflict
		}
		return models.Device{}, err
	}
	return p.GetDevice(ctx, d.OrganizationID, id)
}

func (p *Postgres) UpdateDevice(ctx context.Context, orgID, id uuid.UUID, name *string, status *string, subLocationID **uuid.UUID) (models.Device, error) {
	_, err := p.GetDevice(ctx, orgID, id)
	if err != nil {
		return models.Device{}, err
	}
	if name != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE devices SET name=$1 WHERE id=$2 AND organization_id=$3`, *name, id, orgID); err != nil {
			return models.Device{}, err
		}
	}
	if status != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE devices SET status=$1 WHERE id=$2 AND organization_id=$3`, *status, id, orgID); err != nil {
			return models.Device{}, err
		}
	}
	if subLocationID != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE devices SET sub_location_id=$1 WHERE id=$2 AND organization_id=$3`, *subLocationID, id, orgID); err != nil {
			return models.Device{}, err
		}
	}
	return p.GetDevice(ctx, orgID, id)
}

func (p *Postgres) DeleteDevice(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM devices WHERE id=$1 AND organization_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(row rowScanner) (models.Device, error) {
	var d models.Device
	err := row.Scan(
		&d.ID, &d.OrganizationID, &d.SubLocationID, &d.DeviceIdentifier, &d.Name, &d.Status, &d.CreatedAt,
		&d.LocationID, &d.LocationName, &d.SubLocationName,
	)
	return d, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "duplicate key")
}
