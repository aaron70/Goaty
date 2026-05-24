package validations

import "testing"

func TestStrIsBlank(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", true},
		{"single space", " ", true},
		{"multiple spaces", "   ", true},
		{"tab", "\t", true},
		{"newline", "\n", true},
		{"carriage return", "\r", true},
		{"mixed whitespace", " \t\n\r", true},
		{"unicode whitespace", "\u00a0\u2000", true},
		{"single char", "a", false},
		{"word", "abc", false},
		{"word with spaces", "a b", false},
		{"leading spaces", "  a", false},
		{"trailing spaces", "a  ", false},
		{"surrounding spaces", "  a  ", false},
		{"non-breaking space and char", "\u00a0a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StrIsBlank(tt.input); got != tt.want {
				t.Errorf("StrIsBlank(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
