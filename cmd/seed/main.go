package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/db"
	"github.com/surajkadam7/iot-backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		return err
	}

	orgID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	adminID := uuid.MustParse("00000000-0000-4000-8000-000000000010")
	userID := uuid.MustParse("00000000-0000-4000-8000-000000000011")
	locID := uuid.MustParse("00000000-0000-4000-8000-000000000020")
	subA := uuid.MustParse("00000000-0000-4000-8000-000000000021")
	subB := uuid.MustParse("00000000-0000-4000-8000-000000000022")

	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (id, name, user_limit, status)
		VALUES ($1, 'Acme Manufacturing', 25, 'active')
		ON CONFLICT (id) DO NOTHING`, orgID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, organization_id, cognito_subject, email, role, can_export, status)
		VALUES
			($1, $3, 'local-admin', 'admin@example.com', 'org_admin', true, 'active'),
			($2, $3, 'local-user', 'user@example.com', 'org_user', false, 'active')
		ON CONFLICT (id) DO NOTHING`, adminID, userID, orgID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO locations (id, organization_id, name)
		VALUES ($1, $2, 'Factory 1')
		ON CONFLICT (id) DO NOTHING`, locID, orgID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO sub_locations (id, organization_id, location_id, name)
		VALUES
			($1, $3, $4, 'Line A'),
			($2, $3, $4, 'Line B')
		ON CONFLICT (id) DO NOTHING`, subA, subB, orgID, locID)
	if err != nil {
		return err
	}

	devices := []struct {
		id, ident, name string
		sub            uuid.UUID
	}{
		{"00000000-0000-4000-8000-000000000031", "line-a-01", "Line A · Inlet", subA},
		{"00000000-0000-4000-8000-000000000032", "line-a-02", "Line A · Oven", subA},
		{"00000000-0000-4000-8000-000000000033", "line-b-01", "Line B · Press", subB},
		{"00000000-0000-4000-8000-000000000034", "line-b-02", "Line B · Cooler", subB},
	}
	for _, d := range devices {
		_, err = pool.Exec(ctx, `
			INSERT INTO devices (id, organization_id, sub_location_id, device_identifier, name, status)
			VALUES ($1, $2, $3, $4, $5, 'active')
			ON CONFLICT (device_identifier) DO NOTHING`,
			uuid.MustParse(d.id), orgID, d.sub, d.ident, d.name)
		if err != nil {
			return err
		}
	}

	fmt.Println("Seeded local organization Acme Manufacturing")
	fmt.Println("  org_admin: admin@example.com")
	fmt.Println("  org_user:  user@example.com")
	fmt.Println("MQTT topic pattern: org/00000000-0000-4000-8000-000000000001/device/{device_identifier}/telemetry")
	return nil
}
