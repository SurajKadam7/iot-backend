package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/config"
	"github.com/surajkadam7/iot-backend/internal/db"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
	"github.com/surajkadam7/iot-backend/internal/state"
	"github.com/surajkadam7/iot-backend/internal/ws"
)

type Server struct {
	cfg       config.Config
	log       *slog.Logger
	pool      *pgxpool.Pool
	repo      repository.Store
	store     *state.Store
	validator *auth.Validator
	hub       *ws.Hub
}

func New(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, repo repository.Store, store *state.Store, validator *auth.Validator, hub *ws.Hub) *Server {
	return &Server{cfg: cfg, log: log, pool: pool, repo: repo, store: store, validator: validator, hub: hub}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.logMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(s.cfg.FrontendOrigin, ","),
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:            300,
	}))

	r.Get("/api/health", s.health)
	r.Get("/api/ready", s.ready)
	if s.cfg.AuthMode == "local" {
		r.Post("/api/dev/login", s.devLogin)
	}

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/api/me", s.me)
		r.Get("/api/internal/organizations", s.requirePlatformAdmin(s.listInternalOrganizations))
		r.Get("/api/users", s.requireAdmin(s.listUsers))
		r.Post("/api/users", s.requireAdmin(s.createUser))
		r.Patch("/api/users/{id}", s.requireAdmin(s.patchUser))
		r.Delete("/api/users/{id}", s.requireAdmin(s.deleteUser))
		r.Get("/api/locations", s.listLocations)
		r.Post("/api/locations", s.requireAdmin(s.createLocation))
		r.Get("/api/locations/{id}/sub-locations", s.listSubLocations)
		r.Post("/api/locations/{id}/sub-locations", s.requireAdmin(s.createSubLocation))
		r.Get("/api/devices", s.listDevices)
		r.Get("/api/devices/{id}", s.getDevice)
		r.Post("/api/devices", s.requireAdmin(s.createDevice))
		r.Patch("/api/devices/{id}", s.requireAdmin(s.patchDevice))
		r.Delete("/api/devices/{id}", s.requireAdmin(s.deleteDevice))
	})

	if s.hub != nil {
		r.Get("/ws", s.hub.ServeHTTP)
	}
	s.mountSPA(r)
	return r
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.pool != nil {
		if err := db.Ping(ctx, s.pool); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := auth.BearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		p, err := s.validator.Parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
			return
		}
		user, err := s.lookupUser(r.Context(), p)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "unknown user")
			return
		}
		if !user.IsActive() {
			writeError(w, http.StatusForbidden, "forbidden", "user disabled")
			return
		}
		if user.OrganizationID != p.OrganizationID {
			writeError(w, http.StatusForbidden, "forbidden", "organization mismatch")
			return
		}
		org, err := s.repo.GetOrganization(r.Context(), p.OrganizationID)
		if err != nil || org.Status != models.StatusActive {
			writeError(w, http.StatusForbidden, "forbidden", "organization unavailable")
			return
		}
		p.Email = user.Email
		p.UserID = user.ID
		p.Role = user.Role
		p.CanExport = user.CanExport
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}

func (s *Server) lookupUser(ctx context.Context, p auth.Principal) (models.User, error) {
	user, err := s.repo.GetUserByID(ctx, p.UserID)
	if err == nil {
		return user, nil
	}
	if p.Subject != "" {
		return s.repo.GetUserBySubject(ctx, p.Subject)
	}
	return models.User{}, err
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.FromContext(r.Context())
		if !p.IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden", "admin role required")
			return
		}
		next(w, r)
	}
}

func (s *Server) principal(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p := s.principal(r)
	org, err := s.repo.GetOrganization(r.Context(), p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to load organization")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":           p.UserID,
		"organization_id":  p.OrganizationID,
		"organization_name": org.Name,
		"email":             p.Email,
		"role":              p.Role,
		"can_export":        p.CanExport,
	})
}

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "email is required")
		return
	}
	user, err := s.repo.GetUserByEmail(r.Context(), strings.TrimSpace(body.Email))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unknown user")
		return
	}
	if !user.IsActive() {
		writeError(w, http.StatusForbidden, "forbidden", "user disabled")
		return
	}
	token, err := s.validator.MintLocal(user, s.cfg.JWTExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to issue token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_in": int(s.cfg.JWTExpiry.Seconds()),
	})
}

func (s *Server) listLocations(w http.ResponseWriter, r *http.Request) {
	locs, err := s.repo.ListLocations(r.Context(), s.principal(r).OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list locations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"locations": locationJSONList(locs)})
}

func (s *Server) createLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	name, err := requireName(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	loc, err := s.repo.CreateLocation(r.Context(), s.principal(r).OrganizationID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create location")
		return
	}
	writeJSON(w, http.StatusCreated, locationJSON(loc))
}

func (s *Server) listSubLocations(w http.ResponseWriter, r *http.Request) {
	locID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid location id")
		return
	}
	orgID := s.principal(r).OrganizationID
	if _, err := s.repo.GetLocation(r.Context(), orgID, locID); err != nil {
		writeNotFound(w, err)
		return
	}
	subs, err := s.repo.ListSubLocations(r.Context(), orgID, locID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list sub-locations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sub_locations": subLocationJSONList(subs)})
}

func (s *Server) createSubLocation(w http.ResponseWriter, r *http.Request) {
	locID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid location id")
		return
	}
	orgID := s.principal(r).OrganizationID
	if _, err := s.repo.GetLocation(r.Context(), orgID, locID); err != nil {
		writeNotFound(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	name, err := requireName(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	sub, err := s.repo.CreateSubLocation(r.Context(), orgID, locID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create sub-location")
		return
	}
	writeJSON(w, http.StatusCreated, subLocationJSON(sub))
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.repo.ListDevices(r.Context(), s.principal(r).OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list devices")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": deviceJSONList(devices)})
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid device id")
		return
	}
	d, err := s.repo.GetDevice(r.Context(), s.principal(r).OrganizationID, id)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deviceJSON(d))
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceIdentifier string  `json:"device_identifier"`
		Name             string  `json:"name"`
		SubLocationID    *string `json:"sub_location_id"`
		Status           string  `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	ident, err := requireIdentifier(body.DeviceIdentifier)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	name, err := requireName(body.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	status := models.StatusActive
	if body.Status != "" {
		if body.Status != models.StatusActive && body.Status != models.StatusInactive {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active or inactive")
			return
		}
		status = body.Status
	}
	orgID := s.principal(r).OrganizationID
	var subID *uuid.UUID
	if body.SubLocationID != nil && *body.SubLocationID != "" {
		id, err := uuid.Parse(*body.SubLocationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "invalid sub_location_id")
			return
		}
		if _, err := s.repo.GetSubLocation(r.Context(), orgID, id); err != nil {
			writeNotFound(w, err)
			return
		}
		subID = &id
	}
	d, err := s.repo.CreateDevice(r.Context(), models.Device{
		OrganizationID:   orgID,
		SubLocationID:    subID,
		DeviceIdentifier: ident,
		Name:             name,
		Status:           status,
	})
	if errors.Is(err, repository.ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "device_identifier already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create device")
		return
	}
	writeJSON(w, http.StatusCreated, deviceJSON(d))
}

func (s *Server) patchDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid device id")
		return
	}
	var body struct {
		Name          *string          `json:"name"`
		Status        *string          `json:"status"`
		SubLocationID *json.RawMessage `json:"sub_location_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	var name *string
	if body.Name != nil {
		n, err := requireName(*body.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		name = &n
	}
	if body.Status != nil && *body.Status != models.StatusActive && *body.Status != models.StatusInactive {
		writeError(w, http.StatusBadRequest, "validation_error", "status must be active or inactive")
		return
	}
	orgID := s.principal(r).OrganizationID
	var subPtr **uuid.UUID
	if body.SubLocationID != nil {
		raw := strings.TrimSpace(string(*body.SubLocationID))
		if raw == "null" || raw == `""` {
			var none *uuid.UUID
			subPtr = &none
		} else {
			var sidStr string
			if err := json.Unmarshal(*body.SubLocationID, &sidStr); err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "invalid sub_location_id")
				return
			}
			sid, err := uuid.Parse(sidStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "invalid sub_location_id")
				return
			}
			if _, err := s.repo.GetSubLocation(r.Context(), orgID, sid); err != nil {
				writeNotFound(w, err)
				return
			}
			idCopy := sid
			ptr := &idCopy
			subPtr = &ptr
		}
	}
	d, err := s.repo.UpdateDevice(r.Context(), orgID, id, name, body.Status, subPtr)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deviceJSON(d))
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid device id")
		return
	}
	if err := s.repo.DeleteDevice(r.Context(), s.principal(r).OrganizationID, id); err != nil {
		writeNotFound(w, err)
		return
	}
	s.store.Delete(id)
	w.WriteHeader(http.StatusNoContent)
}

func writeNotFound(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", "request failed")
}

func (s *Server) mountSPA(r chi.Router) {
	dir := s.cfg.WebDistDir
	if dir == "" {
		return
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	index := filepath.Join(dir, "index.html")
	fileServer := http.FileServer(http.Dir(dir))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/ws" {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if !strings.HasPrefix(path, dir) {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			http.ServeFile(w, r, index)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
