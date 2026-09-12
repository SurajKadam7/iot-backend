package archive

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/telemetry"
)

var objectKeyRE = regexp.MustCompile(`^org/([^/]+)/device/(.+)/date=(\d{4}-\d{2}-\d{2})/hour=(\d{2})/[^/]+$`)

func OrgPrefix(orgID uuid.UUID) string {
	return fmt.Sprintf("org/%s/", orgID.String())
}

func DevicePrefix(orgID uuid.UUID, deviceIdentifier string) string {
	return fmt.Sprintf("org/%s/device/%s/", orgID.String(), deviceIdentifier)
}

func PartitionKey(rec telemetry.Record) string {
	ts := rec.TS.UTC()
	return fmt.Sprintf("org/%s/device/%s/date=%s/hour=%s",
		rec.OrganizationID.String(), rec.DeviceIdentifier, ts.Format("2006-01-02"), ts.Format("15"))
}

func ObjectKey(partition, fileID string) string {
	return partition + "/" + fileID + ".csv"
}

func ExportObjectKey(prefix string, orgID, jobID uuid.UUID) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		prefix = "exports"
	}
	return fmt.Sprintf("%s/%s/%s.csv", prefix, orgID.String(), jobID.String())
}

// ParsedKey is the tenant-scoped layout used by archive writes and export listing.
type ParsedKey struct {
	OrganizationID   string
	DeviceIdentifier string
	Hour             time.Time
}

func ParseObjectKey(key string) (ParsedKey, bool) {
	m := objectKeyRE.FindStringSubmatch(key)
	if m == nil {
		return ParsedKey{}, false
	}
	hour, err := time.Parse("2006-01-02 15", m[3]+" "+m[4])
	if err != nil {
		return ParsedKey{}, false
	}
	return ParsedKey{
		OrganizationID:   m[1],
		DeviceIdentifier: m[2],
		Hour:             hour.UTC(),
	}, true
}

func MatchesQuery(key string, orgID uuid.UUID, deviceIDs map[string]struct{}, from, to time.Time) bool {
	parsed, ok := ParseObjectKey(key)
	if !ok {
		return false
	}
	if parsed.OrganizationID != orgID.String() {
		return false
	}
	if len(deviceIDs) > 0 {
		if _, ok := deviceIDs[parsed.DeviceIdentifier]; !ok {
			return false
		}
	}
	fromHour := from.UTC().Truncate(time.Hour)
	toHour := to.UTC().Truncate(time.Hour)
	if parsed.Hour.Before(fromHour) || parsed.Hour.After(toHour) {
		return false
	}
	return true
}
