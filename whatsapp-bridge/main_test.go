package main

import "testing"

func TestIsBareJIDUser(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"phone number", "393487436929", true},
		{"lid user part", "145565165821969", true},
		{"legacy group user part", "393471390814-1414891358", true},
		{"device suffix", "393406271310:38", true},
		{"contact name", "Paola Mamma Marco Sebastiani", false},
		{"group name", "JCP 2016 Sez. Fidenza", false},
		{"group fallback", "Group 120363421665321322", false},
		{"name with digits", "Marco Allenatore 2017 Alseno Calcio", false},
		{"empty", "", false},
		{"separators only", "-:.", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBareJIDUser(tt.in); got != tt.want {
				t.Errorf("isBareJIDUser(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
