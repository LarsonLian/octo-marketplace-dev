package skill

import "testing"

func TestParseLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 20},
		{"0", 20},
		{"-1", 20},
		{"abc", 20},
		{"10", 10},
		{"50", 50},
		{"51", 50},
		{"100", 50},
		{"1", 1},
		{"20", 20},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLimit(tt.input)
			if got != tt.expected {
				t.Errorf("parseLimit(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "123e4567-e89b-12d3-a456-426614174000", true},
		{"valid uppercase", "123E4567-E89B-12D3-A456-426614174000", true},
		{"valid mixed case", "123e4567-E89B-12d3-A456-426614174000", true},
		{"empty", "", false},
		{"too short", "123e4567-e89b-12d3-a456", false},
		{"too long", "123e4567-e89b-12d3-a456-426614174000x", false},
		{"valid uuid with trailing junk", "123e4567-e89b-12d3-a456-426614174000junk", false},
		{"missing dashes", "123e4567e89b12d3a456426614174000xxxx", false},
		{"non-hex characters", "123g4567-e89b-12d3-a456-426614174000", false},
		{"spaces", " 123e4567-e89b-12d3-a456-42661417400", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidUUID(tt.input)
			if got != tt.want {
				t.Errorf("isValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
