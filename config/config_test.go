package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, profile, want string
		files               map[string]string
		wantErr             bool
	}{
		{name: "absent"},
		{name: "yml before yaml", files: map[string]string{"config.yml": "template: first", "config.yaml": "template: second"}, want: "first"},
		{name: "profile yaml before default yml", profile: "talk", files: map[string]string{"config-talk.yaml": "template: profile", "config.yml": "template: default"}, want: "profile"},
		{name: "missing profile falls back to default yaml", profile: "talk", files: map[string]string{"config.yaml": "template: default"}, want: "default"},
		{name: "malformed profile does not fall back", profile: "talk", files: map[string]string{"config-talk.yml": "breaks: [", "config.yml": "template: default"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", home)
			dir := filepath.Join(home, "slidown")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for name, contents := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Load(tc.profile)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "failed to unmarshal config:") {
					t.Fatalf("Load error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || got.Template != tc.want {
				t.Fatalf("Load = %+v, want template %q", got, tc.want)
			}
		})
	}
}
