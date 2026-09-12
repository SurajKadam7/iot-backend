package archive

import (
	"context"
	"encoding/csv"
	"io"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/storage"
)

// Query selects archive objects for one tenant. Callers must set OrganizationID
// from the JWT, never from the request body.
type Query struct {
	OrganizationID uuid.UUID
	DeviceIDs      []string
	From           time.Time
	To             time.Time
}

func deviceSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func ListMatching(ctx context.Context, objects storage.Store, q Query) ([]string, error) {
	if objects == nil {
		return nil, storage.ErrUnavailable
	}
	keys, err := objects.List(ctx, OrgPrefix(q.OrganizationID))
	if err != nil {
		return nil, err
	}
	want := deviceSet(q.DeviceIDs)
	var match []string
	for _, key := range keys {
		if MatchesQuery(key, q.OrganizationID, want, q.From, q.To) {
			match = append(match, key)
		}
	}
	sort.Strings(match)
	return match, nil
}

// MergeCSV writes a single CSV (header once) from archive objects. Duplicate
// MQTT deliveries may appear as duplicate rows; exports tolerate that.
func MergeCSV(ctx context.Context, objects storage.Store, keys []string, q Query, dest io.Writer) error {
	cw := csv.NewWriter(dest)
	if err := cw.Write(Header()); err != nil {
		return err
	}
	want := deviceSet(q.DeviceIDs)
	from := q.From.UTC()
	to := q.To.UTC()
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := appendObject(ctx, objects, key, q.OrganizationID, want, from, to, cw); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func appendObject(ctx context.Context, objects storage.Store, key string, orgID uuid.UUID, want map[string]struct{}, from, to time.Time, cw *csv.Writer) error {
	rc, err := objects.Get(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	cr := csv.NewReader(rc)
	cr.ReuseRecord = true
	first := true
	for {
		row, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if first && IsHeader(row) {
			first = false
			continue
		}
		first = false
		rec, err := DecodeRow(row)
		if err != nil {
			continue
		}
		if rec.OrganizationID != orgID {
			continue
		}
		if want != nil {
			if _, ok := want[rec.DeviceIdentifier]; !ok {
				continue
			}
		}
		ts := rec.TS.UTC()
		if ts.Before(from) || ts.After(to) {
			continue
		}
		if err := cw.Write(EncodeRow(rec)); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}
