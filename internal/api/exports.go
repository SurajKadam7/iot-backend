package api

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/storage"
)

func (s *Server) createExport(w http.ResponseWriter, r *http.Request) {
	if s.objects == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "archive store is not configured")
		return
	}
	var body struct {
		From      string   `json:"from"`
		To        string   `json:"to"`
		DeviceIDs []string `json:"device_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	from, err := parseExportTime(body.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "from must be an RFC3339 timestamp")
		return
	}
	to, err := parseExportTime(body.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "to must be an RFC3339 timestamp")
		return
	}
	if !from.Before(to) {
		writeError(w, http.StatusBadRequest, "validation_error", "from must be before to")
		return
	}
	maxRange := s.cfg.ExportMaxRange
	if maxRange <= 0 {
		maxRange = 31 * 24 * time.Hour
	}
	if to.Sub(from) > maxRange {
		writeError(w, http.StatusBadRequest, "validation_error", "export range is too large")
		return
	}
	p := s.principal(r)
	deviceIDs := make([]string, 0, len(body.DeviceIDs))
	seen := map[string]struct{}{}
	for _, raw := range body.DeviceIDs {
		ident, err := requireIdentifier(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		if _, ok := seen[ident]; ok {
			continue
		}
		seen[ident] = struct{}{}
		d, err := s.repo.GetDeviceByIdentifier(r.Context(), ident)
		if err != nil || d.OrganizationID != p.OrganizationID {
			writeError(w, http.StatusBadRequest, "validation_error", "unknown device_identifier")
			return
		}
		deviceIDs = append(deviceIDs, ident)
	}
	job, err := s.repo.CreateExportJob(r.Context(), models.ExportJob{
		OrganizationID: p.OrganizationID,
		RequestedBy:    p.UserID,
		FromTS:         from,
		ToTS:           to,
		DeviceIDs:      deviceIDs,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create export")
		return
	}
	writeJSON(w, http.StatusAccepted, exportJSON(job, ""))
}

func (s *Server) getExport(w http.ResponseWriter, r *http.Request) {
	job, ok := s.loadExport(w, r)
	if !ok {
		return
	}
	downloadURL := ""
	if job.Status == models.ExportSucceeded {
		downloadURL = "/api/exports/" + job.ID.String() + "/file"
		if job.ObjectKey != "" {
			if p, ok := s.objects.(storage.Presigner); ok {
				if signed, err := p.PresignGet(r.Context(), job.ObjectKey, s.cfg.ExportPresignTTL); err == nil && signed != "" {
					downloadURL = signed
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, exportJSON(job, downloadURL))
}

func (s *Server) downloadExport(w http.ResponseWriter, r *http.Request) {
	job, ok := s.loadExport(w, r)
	if !ok {
		return
	}
	if job.Status != models.ExportSucceeded || job.ObjectKey == "" {
		writeError(w, http.StatusConflict, "conflict", "export is not ready")
		return
	}
	rc, err := s.objects.Get(r.Context(), job.ObjectKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "export file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to read export")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="export-`+job.ID.String()+`.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (s *Server) loadExport(w http.ResponseWriter, r *http.Request) (models.ExportJob, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid export id")
		return models.ExportJob{}, false
	}
	job, err := s.repo.GetExportJob(r.Context(), s.principal(r).OrganizationID, id)
	if err != nil {
		writeNotFound(w, err)
		return models.ExportJob{}, false
	}
	return job, true
}

func parseExportTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func exportJSON(job models.ExportJob, downloadURL string) map[string]any {
	ids := job.DeviceIDs
	if ids == nil {
		ids = []string{}
	}
	out := map[string]any{
		"id":              job.ID,
		"organization_id": job.OrganizationID,
		"status":          job.Status,
		"from":            job.FromTS.UTC().Format(time.RFC3339Nano),
		"to":              job.ToTS.UTC().Format(time.RFC3339Nano),
		"device_ids":      ids,
		"created_at":      job.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      job.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if downloadURL != "" {
		out["download_url"] = downloadURL
	} else {
		out["download_url"] = nil
	}
	if job.ErrorMessage != "" {
		out["error"] = job.ErrorMessage
	} else {
		out["error"] = nil
	}
	return out
}
