package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/surajkadam7/iot-backend/internal/auth"
	"github.com/surajkadam7/iot-backend/internal/models"
	"github.com/surajkadam7/iot-backend/internal/state"
)

const authTimeout = 5 * time.Second

// TenantLookup is the subset of persistence Hub needs. Keeping this small means
// adding export jobs (or other tables) does not ripple into WebSocket tests.
type TenantLookup interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetUserBySubject(ctx context.Context, subject string) (models.User, error)
	GetOrganization(ctx context.Context, id uuid.UUID) (models.Organization, error)
	ListDevices(ctx context.Context, orgID uuid.UUID) ([]models.Device, error)
}

type inbound struct {
	Type      string   `json:"type"`
	Token     string   `json:"token"`
	DeviceIDs []string `json:"device_ids"`
}

type Hub struct {
	upgrader  websocket.Upgrader
	validator *auth.Validator
	repo      TenantLookup
	store     *state.Store
	log       *slog.Logger

	mu      sync.RWMutex
	clients map[*client]struct{}
}

type client struct {
	conn    *websocket.Conn
	orgID   uuid.UUID
	userID  uuid.UUID
	filter  map[uuid.UUID]struct{}
	authed  bool
	writeMu sync.Mutex
}

func NewHub(validator *auth.Validator, repo TenantLookup, store *state.Store, origin string, log *slog.Logger) *Hub {
	return &Hub{
		validator: validator,
		repo:      repo,
		store:     store,
		log:       log,
		clients:   make(map[*client]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				o := r.Header.Get("Origin")
				if o == "" {
					return true
				}
				if origin == "" || origin == "*" {
					return true
				}
				for _, allowed := range strings.Split(origin, ",") {
					if strings.TrimSpace(allowed) == o {
						return true
					}
				}
				return false
			},
		},
	}
}

func (h *Hub) Broadcast(orgID uuid.UUID, reading models.Reading) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.authed || c.orgID != orgID {
			continue
		}
		if c.filter != nil {
			if _, ok := c.filter[reading.DeviceID]; !ok {
				continue
			}
		}
		_ = c.writeJSON(map[string]any{"type": "reading", "reading": reading})
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Debug("ws upgrade failed", "err", err)
		return
	}
	c := &client{conn: conn}
	h.readLoop(c)
}

func (h *Hub) readLoop(c *client) {
	defer func() {
		h.remove(c)
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(authTimeout))
	c.conn.SetReadLimit(16 << 10)
	c.conn.SetPongHandler(func(string) error {
		if c.authed {
			_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg inbound
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = c.writeJSON(map[string]any{"type": "error", "code": "validation_error", "message": "invalid json"})
			continue
		}
		switch msg.Type {
		case "auth":
			if c.authed {
				_ = c.writeJSON(map[string]any{"type": "error", "code": "forbidden", "message": "already authenticated"})
				return
			}
			if err := h.authenticate(c, msg.Token); err != nil {
				_ = c.writeJSON(map[string]any{"type": "error", "code": "unauthorized", "message": "authentication failed"})
				return
			}
			_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		case "filter":
			if !c.authed {
				_ = c.writeJSON(map[string]any{"type": "error", "code": "unauthorized", "message": "authenticate first"})
				return
			}
			h.setFilter(c, msg.DeviceIDs)
		case "ping":
			_ = c.writeJSON(map[string]any{"type": "pong"})
		default:
			if !c.authed {
				_ = c.writeJSON(map[string]any{"type": "error", "code": "unauthorized", "message": "authenticate first"})
				return
			}
		}
	}
}

func (h *Hub) authenticate(c *client, token string) error {
	p, err := h.validator.Parse(token)
	if err != nil {
		return err
	}
	user, err := h.repo.GetUserByID(context.Background(), p.UserID)
	if err != nil {
		user, err = h.repo.GetUserBySubject(context.Background(), p.Subject)
		if err != nil {
			return err
		}
	}
	if !user.IsActive() || user.OrganizationID != p.OrganizationID {
		return auth.ErrUnauthorized
	}
	org, err := h.repo.GetOrganization(context.Background(), p.OrganizationID)
	if err != nil || org.Status != models.StatusActive {
		return auth.ErrUnauthorized
	}
	c.authed = true
	c.orgID = p.OrganizationID
	c.userID = user.ID
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	if err := c.writeJSON(map[string]any{"type": "auth_ok"}); err != nil {
		return err
	}
	h.sendSnapshot(c)
	return nil
}

func (h *Hub) sendSnapshot(c *client) {
	devices, err := h.repo.ListDevices(context.Background(), c.orgID)
	if err != nil {
		h.log.Error("ws snapshot list devices", "err", err)
		return
	}
	ids := make([]uuid.UUID, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, d.ID)
	}
	readings := h.store.Snapshot(ids)
	_ = c.writeJSON(map[string]any{"type": "snapshot", "readings": readings})
}

func (h *Hub) setFilter(c *client, ids []string) {
	if len(ids) == 0 {
		c.filter = nil
		return
	}
	devices, err := h.repo.ListDevices(context.Background(), c.orgID)
	if err != nil {
		return
	}
	allowed := make(map[uuid.UUID]struct{}, len(devices))
	for _, d := range devices {
		allowed[d.ID] = struct{}{}
	}
	filter := make(map[uuid.UUID]struct{})
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		if _, ok := allowed[id]; ok {
			filter[id] = struct{}{}
		}
	}
	c.filter = filter
}

func (c *client) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, b)
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}
