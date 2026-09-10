package state

import (
	"testing"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
)

func TestStoreOverwriteAndSnapshot(t *testing.T) {
	s := New()
	a := uuid.New()
	b := uuid.New()
	s.Set(models.Reading{DeviceID: a, Temperature: 1})
	s.Set(models.Reading{DeviceID: a, Temperature: 2})
	s.Set(models.Reading{DeviceID: b, Temperature: 3})
	got, ok := s.Get(a)
	if !ok || got.Temperature != 2 {
		t.Fatalf("%v %v", got, ok)
	}
	snap := s.Snapshot([]uuid.UUID{a, uuid.New()})
	if len(snap) != 1 || snap[0].Temperature != 2 {
		t.Fatalf("%v", snap)
	}
	s.Delete(a)
	if _, ok := s.Get(a); ok {
		t.Fatal("deleted")
	}
}
