package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleOrgAdmin = "org_admin"
	RoleOrgUser  = "org_user"

	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusInactive = "inactive"
)

type Organization struct {
	ID        uuid.UUID
	Name      string
	UserLimit int
	Status    string
	CreatedAt time.Time
}

type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CognitoSubject  string
	Email          string
	Role           string
	CanExport      bool
	Status         string
	CreatedAt      time.Time
}

type Location struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	CreatedAt      time.Time
}

type SubLocation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	LocationID     uuid.UUID
	Name           string
	CreatedAt      time.Time
}

type Device struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	SubLocationID    *uuid.UUID
	DeviceIdentifier string
	Name             string
	Status           string
	CreatedAt        time.Time

	LocationID       *uuid.UUID
	LocationName     *string
	SubLocationName  *string
}

type Reading struct {
	DeviceID    uuid.UUID `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Pressure    float64   `json:"pressure"`
	Humidity    float64   `json:"humidity"`
	TS          time.Time `json:"ts"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (u User) IsAdmin() bool {
	return u.Role == RoleOrgAdmin
}

func (u User) IsActive() bool {
	return u.Status == StatusActive
}
