package resolver

import (
	"net/url"
	"testing"
)

func TestHttp_ValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		link    *url.URL
		wantErr bool
	}{
		{
			name: "localhost",
			link: &url.URL{
				Host: "localhost",
			},
			wantErr: true,
		}, {
			name: "10.0.0.0/8",
			link: &url.URL{
				Host: "10.0.0.1",
			},
			wantErr: true,
		}, {
			name: "valid",
			link: &url.URL{
				Host: "better-call.dev",
			},
		}, {
			name: "192.0.2.0/24",
			link: &url.URL{
				Host: "192.0.2.1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Http{}
			if err := s.ValidateURL(tt.link); (err != nil) != tt.wantErr {
				t.Errorf("Http.ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
