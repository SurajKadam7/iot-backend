package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/surajkadam7/iot-backend/internal/models"
)

func TestMintAndParseLocal(t *testing.T) {
	v, err := NewValidator("local", "secret", "iot-local", "", "")
	if err != nil {
		t.Fatal(err)
	}
	u := models.User{
		ID:             uuid.MustParse("00000000-0000-4000-8000-000000000010"),
		OrganizationID: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		CognitoSubject:  "local-admin",
		Role:           models.RoleOrgAdmin,
		CanExport:      true,
	}
	tok, err := v.MintLocal(u, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != u.ID || p.OrganizationID != u.OrganizationID || p.Role != models.RoleOrgAdmin || !p.CanExport {
		t.Fatalf("claims %#v", p)
	}
}

func TestRejectsTamperedToken(t *testing.T) {
	v, _ := NewValidator("local", "secret", "iot-local", "", "")
	u := models.User{
		ID: uuid.New(), OrganizationID: uuid.New(), CognitoSubject: "x", Role: models.RoleOrgUser,
	}
	tok, _ := v.MintLocal(u, time.Hour)
	other, _ := NewValidator("local", "other-secret", "iot-local", "", "")
	if _, err := other.Parse(tok); err == nil {
		t.Fatal("expected invalid token")
	}
}

func TestRejectsMissingRole(t *testing.T) {
	if _, err := principalFromClaims(jwt.MapClaims{
		"sub": "x", "user_id": uuid.New().String(), "organization_id": uuid.New().String(),
	}); err == nil {
		t.Fatal("expected error")
	}
}
