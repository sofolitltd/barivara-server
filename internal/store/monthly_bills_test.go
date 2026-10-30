package store

import (
	"testing"
	"time"

	"github.com/sofolitltd/barivara-server/internal/models"
)

func TestLegacyMonthKey(t *testing.T) {
	if got := legacyMonthKey("October 2026"); got != "2026-10" {
		t.Fatalf("legacyMonthKey() = %q, want %q", got, "2026-10")
	}
}

func TestActiveRentForMonthUsesEffectiveRent(t *testing.T) {
	history := []models.TenantRentConfig{
		{Amount: 12000, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Amount: 14000, StartDate: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	}

	amount, ok := activeRentForMonth(history, "October 2026")
	if !ok || amount != 14000 {
		t.Fatalf("activeRentForMonth() = (%d, %t), want (14000, true)", amount, ok)
	}
}
