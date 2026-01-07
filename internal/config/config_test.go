package config

import (
	"testing"
	"time"
)

func TestDestination_Matches(t *testing.T) {
	dest := Destination{
		Alias:    "app-prod-01",
		Hostname: "app-prod-01.internal",
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"app", true},
		{"prod", true},
		{"01", true},
		{"apd", true},      // fuzzy subsequence
		{"internal", true}, // hostname
		{"nope", false},
		{"a-p-0", true},
	}

	for _, tt := range tests {
		if got := dest.Matches(tt.query); got != tt.want {
			t.Errorf("Destination.Matches(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestDestination_CalculateFrecency(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name            string
		count           int
		lastConnectedAt time.Time
		wantMin         float64
	}{
		{"Never", 0, time.Time{}, 0},
		{"Recent (4h)", 10, now.Add(-1 * time.Hour), 1000},
		{"Daily (24h)", 10, now.Add(-12 * time.Hour), 700},
		{"Weekly (7d)", 10, now.Add(-3 * 24 * time.Hour), 500},
		{"Monthly (30d)", 10, now.Add(-15 * 24 * time.Hour), 300},
		{"Old", 10, now.Add(-40 * 24 * time.Hour), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Destination{
				ConnectionCount: tt.count,
				LastConnectedAt: tt.lastConnectedAt,
			}
			got := d.CalculateFrecency()
			if got != tt.wantMin {
				t.Errorf("CalculateFrecency() = %v, want %v", got, tt.wantMin)
			}
		})
	}
}
