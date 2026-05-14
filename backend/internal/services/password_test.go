package services

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSeedAdminPasswordHash(t *testing.T) {
	hash := "$2a$10$3P.Y1sHasju3ekM2pdf2Yu/TE1nNExKf2RwOlio99OgH/fv/L5RDa"
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin12345")); err != nil {
		t.Fatalf("seed admin password hash does not match admin12345: %v", err)
	}
}
