package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/repository"
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.ListUsers(r.Context(), s.principal(r).OrganizationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": userJSONList(users)})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email     string `json:"email"`
		Role      string `json:"role"`
		CanExport bool   `json:"can_export"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	email, err := requireEmail(body.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	role, err := requireRole(body.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	orgID := s.principal(r).OrganizationID
	org, err := s.repo.GetOrganization(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to load organization")
		return
	}
	n, err := s.repo.CountUsers(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to count users")
		return
	}
	if n >= org.UserLimit {
		writeError(w, http.StatusConflict, "user_limit", "organization user limit reached")
		return
	}
	created, err := s.repo.CreateUser(r.Context(), models.User{
		OrganizationID: orgID,
		CognitoSubject: "local-" + uuid.NewString(),
		Email:          email,
		Role:           role,
		CanExport:      body.CanExport,
		Status:         models.StatusActive,
	})
	if errors.Is(err, repository.ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "email already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to create user")
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(created))
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid user id")
		return
	}
	var body struct {
		Role      *string `json:"role"`
		CanExport *bool   `json:"can_export"`
		Status    *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid json")
		return
	}
	orgID := s.principal(r).OrganizationID
	target, err := s.repo.GetUser(r.Context(), orgID, id)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	var role *string
	if body.Role != nil {
		parsed, err := requireRole(*body.Role)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		role = &parsed
	}
	if body.Status != nil {
		if *body.Status != models.StatusActive && *body.Status != models.StatusDisabled {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be active or disabled")
			return
		}
	}
	p := s.principal(r)
	demote := role != nil && *role != models.RoleOrgAdmin && target.Role == models.RoleOrgAdmin && target.Status == models.StatusActive
	disable := body.Status != nil && *body.Status != models.StatusActive && target.Role == models.RoleOrgAdmin && target.Status == models.StatusActive
	if demote || disable {
		if err := s.guardLastAdmin(r, orgID); err != nil {
			writeError(w, http.StatusConflict, "last_admin", err.Error())
			return
		}
	}
	if p.UserID == id {
		if body.Status != nil && *body.Status != models.StatusActive {
			writeError(w, http.StatusForbidden, "forbidden", "cannot disable yourself")
			return
		}
		if role != nil && *role != models.RoleOrgAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "cannot demote yourself")
			return
		}
	}
	updated, err := s.repo.UpdateUser(r.Context(), orgID, id, role, body.CanExport, body.Status)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(updated))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid user id")
		return
	}
	orgID := s.principal(r).OrganizationID
	target, err := s.repo.GetUser(r.Context(), orgID, id)
	if err != nil {
		writeNotFound(w, err)
		return
	}
	if target.Role == models.RoleOrgAdmin && target.Status == models.StatusActive {
		if err := s.guardLastAdmin(r, orgID); err != nil {
			writeError(w, http.StatusConflict, "last_admin", err.Error())
			return
		}
	}
	if s.principal(r).UserID == id {
		writeError(w, http.StatusForbidden, "forbidden", "cannot remove yourself")
		return
	}
	if err := s.repo.DeleteUser(r.Context(), orgID, id); err != nil {
		writeNotFound(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) guardLastAdmin(r *http.Request, orgID uuid.UUID) error {
	n, err := s.repo.CountActiveAdmins(r.Context(), orgID)
	if err != nil {
		return fmt.Errorf("failed to count admins")
	}
	if n <= 1 {
		return fmt.Errorf("cannot remove or demote the last admin")
	}
	return nil
}
