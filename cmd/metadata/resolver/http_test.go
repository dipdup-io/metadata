package resolver

import (
	"net/url"
	"testing"
)

func TestHttp_ValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		link    string
		wantErr bool
	}{
		{
			name:    "localhost",
			link:    "http://localhost:9876",
			wantErr: true,
		}, {
			name:    "localhost 2",
			link:    "http://127.0.0.1:9876",
			wantErr: true,
		}, {
			name:    "10.0.0.0/8",
			link:    "http://10.0.0.1:3333",
			wantErr: true,
		}, {
			name: "valid",
			link: "https://better-call.dev/v1/stats",
		}, {
			name:    "192.0.2.0/24",
			link:    "http://192.0.2.1:1234",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Http{}
			u, err := url.Parse(tt.link)
			if err != nil {
				t.Errorf("Parse: %v", err)
				return
			}

			if err := s.ValidateURL(u); (err != nil) != tt.wantErr {
				t.Errorf("Http.ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
