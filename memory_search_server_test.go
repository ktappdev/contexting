package main

import "testing"

func TestFormatSearchLogQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		max   int
		want  string
	}{
		{
			name:  "trim and normalize whitespace",
			query: "   auth\tmiddleware\nroutes   ",
			max:   120,
			want:  "auth middleware routes",
		},
		{
			name:  "truncate long query",
			query: "abcdefghijklmnopqrstuvwxyz",
			max:   10,
			want:  "abcdefghij...",
		},
		{
			name:  "invalid max uses default",
			query: "abc",
			max:   0,
			want:  "abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSearchLogQuery(tc.query, tc.max)
			if got != tc.want {
				t.Fatalf("formatSearchLogQuery(%q, %d) = %q, want %q", tc.query, tc.max, got, tc.want)
			}
		})
	}
}
