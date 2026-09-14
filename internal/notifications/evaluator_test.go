package notifications

import (
	"testing"
	"time"

	"github.com/wkulhanek/points-tracker/internal/accounts"
	"github.com/wkulhanek/points-tracker/internal/preferences"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestEvaluate(t *testing.T) {
	threshold30 := preferences.Threshold{ID: 1, DaysBefore: 30, Enabled: true}
	threshold7 := preferences.Threshold{ID: 2, DaysBefore: 7, Enabled: true}
	disabled := preferences.Threshold{ID: 3, DaysBefore: 1, Enabled: false}

	account := accounts.Account{ID: 100, Name: "Test Points", ExpirationDate: date("2026-01-31")}

	tests := []struct {
		name       string
		now        time.Time
		thresholds []preferences.Threshold
		sent       []SentNotification
		wantIDs    []int64 // expected threshold IDs in the resulting Pending list
	}{
		{
			name:       "before any threshold is reached",
			now:        date("2025-12-01"),
			thresholds: []preferences.Threshold{threshold30, threshold7},
			wantIDs:    nil,
		},
		{
			name:       "exactly at the 30-day boundary fires",
			now:        date("2026-01-01"), // 2026-01-31 - 30 days
			thresholds: []preferences.Threshold{threshold30, threshold7},
			wantIDs:    []int64{1},
		},
		{
			name:       "day after the 30-day boundary still fires (not yet sent)",
			now:        date("2026-01-02"),
			thresholds: []preferences.Threshold{threshold30, threshold7},
			wantIDs:    []int64{1},
		},
		{
			name:       "both thresholds due at once",
			now:        date("2026-01-24"), // exactly at the 7-day boundary too
			thresholds: []preferences.Threshold{threshold30, threshold7},
			wantIDs:    []int64{1, 2},
		},
		{
			name:       "already sent for current expiration date is skipped",
			now:        date("2026-01-24"),
			thresholds: []preferences.Threshold{threshold30, threshold7},
			sent: []SentNotification{
				{AccountID: 100, ThresholdID: 1, NotifiedExpirationDate: date("2026-01-31")},
			},
			wantIDs: []int64{2},
		},
		{
			name:       "sent record for a stale (old) expiration date does not suppress",
			now:        date("2026-01-24"),
			thresholds: []preferences.Threshold{threshold30, threshold7},
			sent: []SentNotification{
				{AccountID: 100, ThresholdID: 1, NotifiedExpirationDate: date("2025-06-15")},
			},
			wantIDs: []int64{1, 2},
		},
		{
			name:       "disabled threshold never fires",
			now:        date("2026-01-31"),
			thresholds: []preferences.Threshold{disabled},
			wantIDs:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.now, []accounts.Account{account}, tt.thresholds, tt.sent)

			var gotIDs []int64
			for _, p := range got {
				gotIDs = append(gotIDs, p.Threshold.ID)
			}

			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("got threshold IDs %v, want %v", gotIDs, tt.wantIDs)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Fatalf("got threshold IDs %v, want %v", gotIDs, tt.wantIDs)
				}
			}
		})
	}
}

func TestEvaluate_DoesNotExpire(t *testing.T) {
	threshold30 := preferences.Threshold{ID: 1, DaysBefore: 30, Enabled: true}

	// An expiration date far in the past relative to `now`, which would
	// normally make every threshold overdue — DoesNotExpire must still
	// suppress all of them.
	account := accounts.Account{
		ID:             100,
		Name:           "Delta SkyMiles",
		ExpirationDate: date("2020-01-01"),
		DoesNotExpire:  true,
	}

	got := Evaluate(date("2026-01-01"), []accounts.Account{account}, []preferences.Threshold{threshold30}, nil)
	if len(got) != 0 {
		t.Fatalf("got %d pending notifications for a DoesNotExpire account, want 0: %+v", len(got), got)
	}
}
