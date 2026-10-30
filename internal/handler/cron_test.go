package handler

import (
	"errors"
	"testing"
)

func TestShouldSendSMSFallback(t *testing.T) {
	tests := []struct {
		name               string
		hasPushToken       bool
		pushErr            error
		propertySmsEnabled bool
		renterSmsEnabled   bool
		want               bool
	}{
		{name: "both permissions and no app token", propertySmsEnabled: true, renterSmsEnabled: true, want: true},
		{name: "push delivered", hasPushToken: true, propertySmsEnabled: true, renterSmsEnabled: true, want: false},
		{name: "push failed and both permissions", hasPushToken: true, pushErr: errors.New("push failed"), propertySmsEnabled: true, renterSmsEnabled: true, want: true},
		{name: "property permission disabled", renterSmsEnabled: true, want: false},
		{name: "renter permission disabled", propertySmsEnabled: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSendSMSFallback(tt.hasPushToken, tt.pushErr, tt.propertySmsEnabled, tt.renterSmsEnabled); got != tt.want {
				t.Fatalf("shouldSendSMSFallback() = %t, want %t", got, tt.want)
			}
		})
	}
}
