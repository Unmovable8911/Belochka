//go:build !windows && !darwin

package main

import "testing"

func TestHasDesktop(t *testing.T) {
	tests := []struct {
		name     string
		display  string
		wayland  string
		expected bool
	}{
		{"both empty", "", "", false},
		{"DISPLAY set", ":0", "", true},
		{"WAYLAND_DISPLAY set", "", "wayland-0", true},
		{"both set", ":0", "wayland-0", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISPLAY", tc.display)
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			if got := hasDesktop(); got != tc.expected {
				t.Errorf("hasDesktop() = %v, want %v", got, tc.expected)
			}
		})
	}
}
