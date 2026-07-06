package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"valid email", "user@example.com", true},
		{"missing @", "userexample.com", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidEmail(tt.email))
		})
	}
}

func TestGetNameFromEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"standard email", "john.doe@example.com", "John"},
		{"numeric prefix", "123abc@x.com", "Abc"},
		{"empty local part", "@x.com", ""},
		{"no alpha chars", "123@x.com", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetNameFromEmail(tt.email))
		})
	}
}
