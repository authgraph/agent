package agent

import (
	"testing"
)

func TestParseEntity(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"user:alice", []string{"user", "alice"}},
		{"document:readme", []string{"document", "readme"}},
		{"service:agent-1", []string{"service", "agent-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseEntity(tt.input)
			if got == nil {
				t.Fatalf("parseEntity(%q) = nil, want %v", tt.input, tt.want)
			}
			if got[0] != tt.want[0] || got[1] != tt.want[1] {
				t.Errorf("parseEntity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEntityInvalid(t *testing.T) {
	tests := []string{
		"useralice",
		":alice",
		"user:",
		"",
		"no-colon-here",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			got := parseEntity(input)
			if got != nil {
				t.Errorf("parseEntity(%q) = %v, want nil", input, got)
			}
		})
	}
}

func TestParseEntityPair(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{"basic check", "can user:alice read document:readme", false},
		{"check without can", "user:alice read document:readme", false},
		{"no entities", "hello world", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEntityPair(tt.input)
			if tt.wantNil && got != nil {
				t.Errorf("parseEntityPair(%q) = %+v, want nil", tt.input, got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("parseEntityPair(%q) = nil, want non-nil", tt.input)
			}
		})
	}
}
