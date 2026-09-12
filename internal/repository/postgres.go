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
	ListOrganizations(ctx context.Context) ([]models.Organization, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserBySubject(ctx context.Context, subject string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUser(ctx context.Context, orgID, id uuid.UUID) (models.User, error)
	ListUsers(ctx context.Context, orgID uuid.UUID) ([]models.User, error)
	ListAllUsers(ctx context.Context) ([]models.User, error)
	CountUsers(ctx context.Context, orgID uuid.UUID) (int, error)
	CountActiveAdmins(ctx context.Context, orgID uuid.UUID) (int, error)
	CreateUser(ctx context.Context, u models.User) (models.User, error)
	UpdateUser(ctx context.Context, orgID, id uuid.UUID, role *string, canExport *bool, status *string) (models.User, error)
	DeleteUser(ctx context.Context, orgID, id uuid.UUID) error

	ListLocations(ctx context.Context, orgID uuid.UUID) ([]models.Location, error)
	ListAllLocations(ctx context.Context) ([]models.Location, error)
	CreateLocation(ctx context.Context, orgID uuid.UUID, name string) (models.Location, error)
	GetLocation(ctx context.Context, orgID, id uuid.UUID) (models.Location, error)

	ListSubLocations(ctx context.Context, orgID, locationID uuid.UUID) ([]models.SubLocation, error)
	ListAllSubLocations(ctx context.Context) ([]models.SubLocation, error)
	CreateSubLocation(ctx context.Context, orgID, locationID uuid.UUID, name string) (models.SubLocation, error)
	GetSubLocation(ctx context.Context, orgID, id uuid.UUID) (models.SubLocation, error)

	ListDevices(ctx context.Context, orgID uuid.UUID) ([]models.Device, error)
	ListAllDevices(ctx context.Context) ([]models.Device, error)
	GetDevice(ctx context.Context, orgID, id uuid.UUID) (models.Device, error)
	GetDeviceByIdentifier(ctx context.Context, identifier string) (models.Device, error)
	CreateDevice(ctx context.Context, d models.Device) (models.Device, error)
	UpdateDevice(ctx context.Context, orgID, id uuid.UUID, name *string, status *string, subLocationID **uuid.UUID) (models.Device, error)
	DeleteDevice(ctx context.Context, orgID, id uuid.UUID) error

	CreateExportJob(ctx context.Context, job models.ExportJob) (models.ExportJob, error)
	GetExportJob(ctx context.Context, orgID, id uuid.UUID) (models.ExportJob, error)
	ClaimNextExportJob(ctx context.Context) (models.ExportJob, error)
	FinishExportJob(ctx context.Context, id uuid.UUID, status, objectKey, errorMessage string) error
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

func (p *Postgres) ListOrganizations(ctx context.Context) ([]models.Organization, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, user_limit, status, created_at
		FROM organizations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Organization
	for rows.Next() {
		var o models.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.UserLimit, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if out == nil {
		out = []models.Organization{}
	}
	return out, rows.Err()
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

func (p *Postgres) GetUser(ctx context.Context, orgID, id uuid.UUID) (models.User, error) {
	return p.scanUser(p.pool.QueryRow(ctx, userSelect+" WHERE id=$1 AND organization_id=$2", id, orgID))
}

func (p *Postgres) ListUsers(ctx context.Context, orgID uuid.UUID) ([]models.User, error) {
	rows, err := p.pool.Query(ctx, userSelect+" WHERE organization_id=$1 ORDER BY email", orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []models.User{}
	}
	return out, rows.Err()
}

func (p *Postgres) ListAllUsers(ctx context.Context) ([]models.User, error) {
	rows, err := p.pool.Query(ctx, userSelect+" ORDER BY email")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []models.User{}
	}
	return out, rows.Err()
}

func (p *Postgres) CountUsers(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE organization_id=$1`, orgID).Scan(&n)
	return n, err
}

func (p *Postgres) CountActiveAdmins(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE organization_id=$1 AND role=$2 AND status=$3`,
		orgID, models.RoleOrgAdmin, models.StatusActive).Scan(&n)
	return n, err
}

func (p *Postgres) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	created, err := p.scanUser(p.pool.QueryRow(ctx, `
		INSERT INTO users (organization_id, cognito_subject, email, role, can_export, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, cognito_subject, email, role, can_export, status, created_at`,
		u.OrganizationID, u.CognitoSubject, u.Email, u.Role, u.CanExport, u.Status))
	if err != nil {
		if isUniqueViolation(err) {
			return models.User{}, ErrConflict
		}
		return models.User{}, err
	}
	return created, nil
}

func (p *Postgres) UpdateUser(ctx context.Context, orgID, id uuid.UUID, role *string, canExport *bool, status *string) (models.User, error) {
	if _, err := p.GetUser(ctx, orgID, id); err != nil {
		return models.User{}, err
	}
	if role != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE users SET role=$1 WHERE id=$2 AND organization_id=$3`, *role, id, orgID); err != nil {
			return models.User{}, err
		}
	}
	if canExport != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE users SET can_export=$1 WHERE id=$2 AND organization_id=$3`, *canExport, id, orgID); err != nil {
			return models.User{}, err
		}
	}
	if status != nil {
		if _, err := p.pool.Exec(ctx, `UPDATE users SET status=$1 WHERE id=$2 AND organization_id=$3`, *status, id, orgID); err != nil {
			return models.User{}, err
		}
	}
	return p.GetUser(ctx, orgID, id)
}

func (p *Postgres) DeleteUser(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM users WHERE id=$1 AND organization_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const userSelect = `SELECT id, organization_id, cognito_subject, email, role, can_export, status, created_at FROM users`

func (p *Postgres) scanUser(row pgx.Row) (models.User, error) {
	u, err := scanUserRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func scanUserRow(row rowScanner) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.OrganizationID, &u.CognitoSubject, &u.Email, &u.Role, &u.CanExport, &u.Status, &u.CreatedAt)
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

func (p *Postgres) ListAllLocations(ctx context.Context) ([]models.Location, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, organization_id, name, created_at
		FROM locations ORDER BY name`)
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

func (p *Postgres) ListAllSubLocations(ctx context.Context) ([]models.SubLocation, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, organization_id, location_id, name, created_at
		FROM sub_locations ORDER BY name`)
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

func (p *Postgres) ListAllDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelect+` ORDER BY d.name`)
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

const exportSelect = `
SELECT id, organization_id, requested_by, status, from_ts, to_ts, device_ids, object_key, error_message, created_at, updated_at
FROM export_jobs`

func (p *Postgres) CreateExportJob(ctx context.Context, job models.ExportJob) (models.ExportJob, error) {
	if job.DeviceIDs == nil {
		job.DeviceIDs = []string{}
	}
	row := p.pool.QueryRow(ctx, `
		INSERT INTO export_jobs (organization_id, requested_by, status, from_ts, to_ts, device_ids)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, organization_id, requested_by, status, from_ts, to_ts, device_ids, object_key, error_message, created_at, updated_at`,
		job.OrganizationID, job.RequestedBy, models.ExportQueued, job.FromTS, job.ToTS, job.DeviceIDs)
	return scanExportJob(row)
}

func (p *Postgres) GetExportJob(ctx context.Context, orgID, id uuid.UUID) (models.ExportJob, error) {
	job, err := scanExportJob(p.pool.QueryRow(ctx, exportSelect+` WHERE id=$1 AND organization_id=$2`, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ExportJob{}, ErrNotFound
	}
	return job, err
}

func (p *Postgres) ClaimNextExportJob(ctx context.Context) (models.ExportJob, error) {
	job, err := scanExportJob(p.pool.QueryRow(ctx, `
		UPDATE export_jobs SET status=$1, updated_at=now()
		WHERE id = (
			SELECT id FROM export_jobs
			WHERE status=$2
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, organization_id, requested_by, status, from_ts, to_ts, device_ids, object_key, error_message, created_at, updated_at`,
		models.ExportRunning, models.ExportQueued))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ExportJob{}, ErrNotFound
	}
	return job, err
}

func (p *Postgres) FinishExportJob(ctx context.Context, id uuid.UUID, status, objectKey, errorMessage string) error {
	var key any
	if objectKey != "" {
		key = objectKey
	}
	var msg any
	if errorMessage != "" {
		msg = errorMessage
	}
	tag, err := p.pool.Exec(ctx, `
		UPDATE export_jobs
		SET status=$1, object_key=$2, error_message=$3, updated_at=now()
		WHERE id=$4`, status, key, msg, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanExportJob(row rowScanner) (models.ExportJob, error) {
	var j models.ExportJob
	var objectKey *string
	var errMsg *string
	err := row.Scan(
		&j.ID, &j.OrganizationID, &j.RequestedBy, &j.Status, &j.FromTS, &j.ToTS, &j.DeviceIDs,
		&objectKey, &errMsg, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return models.ExportJob{}, err
	}
	if objectKey != nil {
		j.ObjectKey = *objectKey
	}
	if errMsg != nil {
		j.ErrorMessage = *errMsg
	}
	if j.DeviceIDs == nil {
		j.DeviceIDs = []string{}
	}
	return j, nil
}
