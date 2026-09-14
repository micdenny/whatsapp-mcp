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

func TestExtractDirectPathFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"keeps the query string",
			"https://mmg.whatsapp.net/v/t62.7119-24/802309742_1113672974664032_288117231437204654_n.enc?ccb=11-4&oh=01_Q5Aa5gFv&oe=6ACB3A24&_nc_sid=5e03e0&mms3=true",
			"/v/t62.7119-24/802309742_1113672974664032_288117231437204654_n.enc?ccb=11-4&oh=01_Q5Aa5gFv&oe=6ACB3A24&_nc_sid=5e03e0&mms3=true",
		},
		{
			"no query string",
			"https://mmg.whatsapp.net/v/t62.7118-24/13812002_698058036224062_n.enc",
			"/v/t62.7118-24/13812002_698058036224062_n.enc",
		},
		{
			"host outside .net",
			"https://media-mxp1-1.cdn.whatsapp.com/v/t62.7118-24/file.enc?ccb=11-4",
			"/v/t62.7118-24/file.enc?ccb=11-4",
		},
		{"already a direct path", "/v/t62.7118-24/file.enc?ccb=11-4", "/v/t62.7118-24/file.enc?ccb=11-4"},
		{"no path", "https://mmg.whatsapp.net", "https://mmg.whatsapp.net"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDirectPathFromURL(tt.in); got != tt.want {
				t.Errorf("extractDirectPathFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
