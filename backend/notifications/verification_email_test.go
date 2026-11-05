package notifications

import "testing"

func TestSMTPSTARTTLSRequired(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		setting string
		want    bool
		wantErr bool
	}{
		{name: "defaults to required", host: "smtp.example.com", want: true},
		{name: "explicitly required", host: "smtp.example.com", setting: "true", want: true},
		{name: "localhost can disable", host: "localhost", setting: "false"},
		{name: "loopback IP can disable", host: "127.0.0.1", setting: "false"},
		{name: "IPv6 loopback can disable", host: "::1", setting: "false"},
		{name: "remote host cannot disable", host: "smtp.example.com", setting: "false", wantErr: true},
		{name: "invalid setting", host: "localhost", setting: "no", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := smtpSTARTTLSRequired(test.host, test.setting)
			if (err != nil) != test.wantErr {
				t.Fatalf("smtpSTARTTLSRequired() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("smtpSTARTTLSRequired() = %v, want %v", got, test.want)
			}
		})
	}
}
