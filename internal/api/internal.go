package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/models"
)

func (s *Server) requirePlatformAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.FromContext(r.Context())
		if !p.IsPlatformAdmin() {
			writeError(w, http.StatusForbidden, "forbidden", "platform admin required")
			return
		}
		next(w, r)
	}
}

func (s *Server) listInternalOrganizations(w http.ResponseWriter, r *http.Request) {
	orgs, err := s.repo.ListOrganizations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list organizations")
		return
	}
	users, err := s.repo.ListAllUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list users")
		return
	}
	locs, err := s.repo.ListAllLocations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list locations")
		return
	}
	subs, err := s.repo.ListAllSubLocations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list sub-locations")
		return
	}
	devices, err := s.repo.ListAllDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list devices")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"organizations": organizationOverviewList(orgs, users, locs, subs, devices),
	})
}

func organizationOverviewList(
	orgs []models.Organization,
	users []models.User,
	locs []models.Location,
	subs []models.SubLocation,
	devices []models.Device,
) []map[string]any {
	usersByOrg := map[uuid.UUID][]models.User{}
	for _, u := range users {
		usersByOrg[u.OrganizationID] = append(usersByOrg[u.OrganizationID], u)
	}
	locsByOrg := map[uuid.UUID][]models.Location{}
	for _, l := range locs {
		locsByOrg[l.OrganizationID] = append(locsByOrg[l.OrganizationID], l)
	}
	subsByLoc := map[uuid.UUID][]models.SubLocation{}
	for _, s := range subs {
		subsByLoc[s.LocationID] = append(subsByLoc[s.LocationID], s)
	}
	subDeviceCount := map[uuid.UUID]int{}
	unassigned := map[uuid.UUID]int{}
	orgDeviceCount := map[uuid.UUID]int{}
	for _, d := range devices {
		orgDeviceCount[d.OrganizationID]++
		if d.SubLocationID == nil {
			unassigned[d.OrganizationID]++
			continue
		}
		subDeviceCount[*d.SubLocationID]++
	}

	out := make([]map[string]any, 0, len(orgs))
	for _, o := range orgs {
		orgUsers := usersByOrg[o.ID]
		if orgUsers == nil {
			orgUsers = []models.User{}
		}
		orgLocs := locsByOrg[o.ID]
		if orgLocs == nil {
			orgLocs = []models.Location{}
		}
		sort.Slice(orgLocs, func(i, j int) bool { return orgLocs[i].Name < orgLocs[j].Name })

		locJSON := make([]map[string]any, 0, len(orgLocs))
		for _, loc := range orgLocs {
			children := subsByLoc[loc.ID]
			sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
			subJSON := make([]map[string]any, 0, len(children))
			locCount := 0
			for _, sub := range children {
				n := subDeviceCount[sub.ID]
				locCount += n
				subJSON = append(subJSON, map[string]any{
					"id":           sub.ID,
					"name":         sub.Name,
					"device_count": n,
				})
			}
			locJSON = append(locJSON, map[string]any{
				"id":            loc.ID,
				"name":          loc.Name,
				"device_count":  locCount,
				"sub_locations": subJSON,
			})
		}

		userJSON := make([]map[string]any, 0, len(orgUsers))
		for _, u := range orgUsers {
			userJSON = append(userJSON, map[string]any{
				"id":     u.ID,
				"email":  u.Email,
				"role":   u.Role,
				"status": u.Status,
			})
		}

		created := ""
		if !o.CreatedAt.IsZero() {
			created = o.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		out = append(out, map[string]any{
			"id":                      o.ID,
			"name":                    o.Name,
			"status":                  o.Status,
			"user_limit":              o.UserLimit,
			"created_at":              created,
			"user_count":              len(orgUsers),
			"device_count":            orgDeviceCount[o.ID],
			"unassigned_device_count": unassigned[o.ID],
			"users":                   userJSON,
			"locations":               locJSON,
		})
	}
	return out
}
