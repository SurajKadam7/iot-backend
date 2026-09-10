package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
)

type contextKey string

const principalKey contextKey = "principal"

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrUnauthorized = errors.New("unauthorized")
)

type Principal struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	CanExport      bool
	Subject        string
	Email          string
}

func (p Principal) IsAdmin() bool {
	return p.Role == models.RoleOrgAdmin
}

type Validator struct {
	mode      string
	secret    []byte
	issuer    string
	audience  string
	keyfunc   jwt.Keyfunc
}

func NewValidator(mode, secret, issuer, audience, jwksURL string) (*Validator, error) {
	v := &Validator{
		mode:     mode,
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
	}
	if mode == "cognito" {
		k, err := keyfunc.NewDefault([]string{jwksURL})
		if err != nil {
			return nil, fmt.Errorf("jwks: %w", err)
		}
		v.keyfunc = k.Keyfunc
	}
	return v, nil
}

func (v *Validator) Parse(tokenString string) (Principal, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg(), jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if v.issuer != "" {
		parser = jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg(), jwt.SigningMethodRS256.Alg()}),
			jwt.WithExpirationRequired(),
			jwt.WithIssuer(v.issuer),
		)
	}

	keyfunc := v.secretKeyfunc
	if v.mode == "cognito" {
		keyfunc = v.keyfunc
	}

	token, err := parser.Parse(tokenString, keyfunc)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return Principal{}, ErrInvalidToken
	}
	if v.audience != "" {
		auds, _ := claims.GetAudience()
		found := false
		for _, a := range auds {
			if a == v.audience {
				found = true
				break
			}
		}
		if !found {
			if cid, _ := stringClaim(claims, "client_id"); cid != v.audience {
				return Principal{}, fmt.Errorf("%w: audience", ErrInvalidToken)
			}
		}
	}
	return principalFromClaims(claims)
}

func (v *Validator) secretKeyfunc(t *jwt.Token) (any, error) {
	if t.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing method")
	}
	return v.secret, nil
}

func (v *Validator) MintLocal(user models.User, ttl time.Duration) (string, error) {
	if v.mode != "local" {
		return "", fmt.Errorf("local mint disabled")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":             v.issuer,
		"sub":             user.CognitoSubject,
		"user_id":         user.ID.String(),
		"organization_id": user.OrganizationID.String(),
		"role":            user.Role,
		"can_export":      user.CanExport,
		"iat":             now.Unix(),
		"exp":             now.Add(ttl).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(v.secret)
}

func principalFromClaims(claims jwt.MapClaims) (Principal, error) {
	sub, _ := stringClaim(claims, "sub")
	userIDStr, _ := firstString(claims, "user_id", "custom:user_id")
	if userIDStr == "" {
		userIDStr = sub
	}
	orgStr, _ := firstString(claims, "organization_id", "custom:organization_id")
	role, _ := firstString(claims, "role", "custom:role")
	if role != models.RoleOrgAdmin && role != models.RoleOrgUser {
		return Principal{}, fmt.Errorf("%w: role", ErrInvalidToken)
	}
	orgID, err := uuid.Parse(orgStr)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: organization_id", ErrInvalidToken)
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: user_id", ErrInvalidToken)
	}
	canExport, err := boolClaim(claims, "can_export", "custom:can_export")
	if err != nil {
		return Principal{}, fmt.Errorf("%w: can_export", ErrInvalidToken)
	}
	return Principal{
		UserID:         userID,
		OrganizationID: orgID,
		Role:           role,
		CanExport:      canExport,
		Subject:        sub,
	}, nil
}

func firstString(claims jwt.MapClaims, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := stringClaim(claims, k); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

func stringClaim(claims jwt.MapClaims, key string) (string, bool) {
	v, ok := claims[key]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return fmt.Sprint(t), true
	}
}

func boolClaim(claims jwt.MapClaims, keys ...string) (bool, error) {
	for _, key := range keys {
		v, ok := claims[key]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t, nil
		case string:
			switch strings.ToLower(t) {
			case "true", "1", "yes":
				return true, nil
			case "false", "0", "no":
				return false, nil
			}
		case json.Number:
			i, err := t.Int64()
			return i != 0, err
		case float64:
			return t != 0, nil
		}
	}
	return false, fmt.Errorf("missing")
}

func BearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}
	typ, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(typ, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}
