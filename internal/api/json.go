package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/surajkadam7/iot-backend/internal/models"
)

var identifierRE = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func requireName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if len(name) > 200 {
		return "", fmt.Errorf("name is too long")
	}
	return name, nil
}

func requireIdentifier(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !identifierRE.MatchString(id) {
		return "", fmt.Errorf("device_identifier must be 1-128 characters of letters, numbers, . _ : -")
	}
	return id, nil
}

func locationJSON(l models.Location) map[string]any {
	return map[string]any{
		"id":              l.ID,
		"organization_id": l.OrganizationID,
		"name":            l.Name,
		"created_at":      l.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func locationJSONList(list []models.Location) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, l := range list {
		out = append(out, locationJSON(l))
	}
	return out
}

func subLocationJSON(s models.SubLocation) map[string]any {
	return map[string]any{
		"id":              s.ID,
		"organization_id": s.OrganizationID,
		"location_id":     s.LocationID,
		"name":            s.Name,
		"created_at":      s.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func subLocationJSONList(list []models.SubLocation) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, subLocationJSON(s))
	}
	return out
}

func deviceJSON(d models.Device) map[string]any {
	m := map[string]any{
		"id":                d.ID,
		"organization_id":   d.OrganizationID,
		"device_identifier":  d.DeviceIdentifier,
		"name":              d.Name,
		"status":            d.Status,
		"created_at":        d.CreatedAt.UTC().Format(time.RFC3339Nano),
		"mqtt_topic":        "org/" + d.OrganizationID.String() + "/device/" + d.DeviceIdentifier + "/telemetry",
	}
	if d.SubLocationID != nil {
		m["sub_location_id"] = d.SubLocationID.String()
	} else {
		m["sub_location_id"] = nil
	}
	if d.LocationID != nil {
		m["location_id"] = d.LocationID.String()
	} else {
		m["location_id"] = nil
	}
	if d.LocationName != nil {
		m["location_name"] = *d.LocationName
	} else {
		m["location_name"] = nil
	}
	if d.SubLocationName != nil {
		m["sub_location_name"] = *d.SubLocationName
	} else {
		m["sub_location_name"] = nil
	}
	return m
}

func deviceJSONList(list []models.Device) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, deviceJSON(d))
	}
	return out
}
