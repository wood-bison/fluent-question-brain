package config

import "testing"

func TestFromEnvRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}

func TestFromEnvAcceptsPostgresURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("HTTP_ADDR", ":9090")
	c, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if c.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q", c.HTTPAddr)
	}
}
