package buildmeta

import "testing"

func TestValidateTLSVerify(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default empty", input: "", want: "on"},
		{name: "on", input: "on", want: "on"},
		{name: "off", input: "off", want: "off"},
		{name: "uppercase off", input: "OFF", want: "off"},
		{name: "invalid", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevURL := ControlPlaneURL
			prevVerify := TLSVerify
			prevVersion := SoftVersion
			ControlPlaneURL = "http://127.0.0.1:19090"
			TLSVerify = tt.input
			SoftVersion = "1.0.0"
			t.Cleanup(func() {
				ControlPlaneURL = prevURL
				TLSVerify = prevVerify
				SoftVersion = prevVersion
			})

			err := Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Validate unexpectedly succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if TLSVerify != tt.want {
				t.Fatalf("TLSVerify = %q, want %q", TLSVerify, tt.want)
			}
		})
	}
}

func TestValidateRejectsMissingVersion(t *testing.T) {
	prevURL := ControlPlaneURL
	prevVersion := SoftVersion
	ControlPlaneURL = "http://127.0.0.1:19090"
	SoftVersion = "dev"
	t.Cleanup(func() {
		ControlPlaneURL = prevURL
		SoftVersion = prevVersion
	})

	if err := Validate(); err == nil {
		t.Fatal("Validate unexpectedly succeeded")
	}
}

func TestTLSVerifyEnabled(t *testing.T) {
	t.Parallel()

	SetTLSVerifyForTest(t, "on")
	if !TLSVerifyEnabled() {
		t.Fatal("expected on to enable TLS verify")
	}

	SetTLSVerifyForTest(t, "off")
	if TLSVerifyEnabled() {
		t.Fatal("expected off to disable TLS verify")
	}
}
