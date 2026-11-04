package migrations

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewRunnerRejectsNilDatabase(t *testing.T) {
	_, err := NewRunner(nil)
	if !errors.Is(err, ErrNilDB) {
		t.Fatalf("NewRunner(nil) error = %v, want ErrNilDB", err)
	}
}

func TestNewRunnerRejectsNonPostgresDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:migrations_runner?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	_, err = NewRunner(db)
	if !errors.Is(err, ErrUnsupportedDialect) {
		t.Fatalf("NewRunner(sqlite) error = %v, want ErrUnsupportedDialect", err)
	}
}

func TestEmbeddedMigrationFiles(t *testing.T) {
	data, err := fs.ReadFile(Files(), "postgres/0001_init_permission_module.sql")
	if err != nil {
		t.Fatalf("read embedded migration: %v", err)
	}
	sql := string(data)
	for _, marker := range []string{"-- +goose Up", "-- +goose Down", "CREATE TABLE sys_role", "CREATE TABLE sys_menu", "CREATE TABLE sys_api", "CREATE TABLE sys_user"} {
		if !strings.Contains(sql, marker) {
			t.Errorf("embedded migration does not contain %q", marker)
		}
	}
	if strings.Contains(sql, "sys_user_post") {
		t.Fatal("embedded migration still contains the removed sys_user_post table")
	}

	for _, marker := range []string{
		`FOREIGN KEY (parent_id) REFERENCES sys_api (id)`,
		`"group" IS NOT NULL AND "group" != '' AND method = '' AND path = ''`,
		`CREATE UNIQUE INDEX idx_sys_api_group`,
	} {
		if !strings.Contains(sql, marker) {
			t.Errorf("initial migration does not support nested API groups: missing %q", marker)
		}
	}
}

func TestValidateRunnerContext(t *testing.T) {
	if err := validateRunnerContext(nil, context.Background()); !errors.Is(err, ErrNilDB) {
		t.Fatalf("nil runner error = %v, want ErrNilDB", err)
	}
	if err := validateRunnerContext(&Runner{}, context.Background()); !errors.Is(err, ErrNilDB) {
		t.Fatalf("empty runner error = %v, want ErrNilDB", err)
	}
	if err := validateRunnerContext(&Runner{provider: &goose.Provider{}}, nil); !errors.Is(err, ErrNilContext) {
		t.Fatalf("nil context error = %v, want ErrNilContext", err)
	}
}
