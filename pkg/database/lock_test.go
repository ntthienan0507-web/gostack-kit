package database

import (
	"testing"

	"gorm.io/gorm/clause"
)

func TestForUpdate(t *testing.T) {
	got := ForUpdate()
	want := clause.Locking{Strength: "UPDATE"}

	if got != want {
		t.Errorf("ForUpdate() = %+v, want %+v", got, want)
	}
}

func TestForUpdateNoWait(t *testing.T) {
	got := ForUpdateNoWait()

	if got.Strength != "UPDATE" {
		t.Errorf("Strength = %q, want %q", got.Strength, "UPDATE")
	}
	if got.Options != "NOWAIT" {
		t.Errorf("Options = %q, want %q", got.Options, "NOWAIT")
	}
}

func TestForUpdateSkipLocked(t *testing.T) {
	got := ForUpdateSkipLocked()

	if got.Strength != "UPDATE" {
		t.Errorf("Strength = %q, want %q", got.Strength, "UPDATE")
	}
	if got.Options != "SKIP LOCKED" {
		t.Errorf("Options = %q, want %q", got.Options, "SKIP LOCKED")
	}
}

func TestForShare(t *testing.T) {
	got := ForShare()
	want := clause.Locking{Strength: "SHARE"}

	if got != want {
		t.Errorf("ForShare() = %+v, want %+v", got, want)
	}
}
