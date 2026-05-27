package service

import "testing"

func TestGoogleTokenInfoIsEmailVerified(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{name: "bool true", value: true, want: true},
		{name: "bool false", value: false, want: false},
		{name: "string true", value: "true", want: true},
		{name: "string true uppercase", value: "TRUE", want: true},
		{name: "string false", value: "false", want: false},
		{name: "missing", value: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := googleTokenInfo{EmailVerified: tt.value}
			if got := info.isEmailVerified(); got != tt.want {
				t.Fatalf("isEmailVerified() = %v, want %v", got, tt.want)
			}
		})
	}
}
