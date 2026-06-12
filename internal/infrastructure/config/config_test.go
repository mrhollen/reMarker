package config

import (
	"os"
	"testing"
	"time"
)

func TestEnvConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"EnvPassword", EnvPassword, "REMARKABLE_PASSWORD"},
		{"EnvHost", EnvHost, "REMARKABLE_HOST"},
		{"EnvPort", EnvPort, "REMARKABLE_PORT"},
		{"EnvUser", EnvUser, "REMARKABLE_USER"},
		{"EnvSyncDir", EnvSyncDir, "REMARKER_SYNC_DIR"},
		{"EnvSyncInterval", EnvSyncInterval, "SYNC_INTERVAL"},
		{"EnvConnectionTimeout", EnvConnectionTimeout, "REMARKABLE_CONNECTION_TIMEOUT"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Unset all config env vars so defaults apply
	for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval, EnvConnectionTimeout} {
		t.Setenv(env, "")
	}

	c := Load()

	if c.Host != "10.11.99.1" {
		t.Errorf("Host = %q, want %q", c.Host, "10.11.99.1")
	}
	if c.Port != 22 {
		t.Errorf("Port = %d, want %d", c.Port, 22)
	}
	if c.User != "root" {
		t.Errorf("User = %q, want %q", c.User, "root")
	}
	if c.Password != "" {
		t.Errorf("Password = %q, want empty", c.Password)
	}
	if c.SyncDir != "./documents" {
		t.Errorf("SyncDir = %q, want %q", c.SyncDir, "./documents")
	}
	if c.SyncInterval != 5*time.Minute {
		t.Errorf("SyncInterval = %v, want %v", c.SyncInterval, 5*time.Minute)
	}
	if c.ConnectionTimeout != 30*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", c.ConnectionTimeout, 30*time.Second)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "override host",
			env:  map[string]string{EnvHost: "192.168.1.1"},
			want: Config{Host: "192.168.1.1", Port: 22, User: "root", Password: "", SyncDir: "./documents", SyncInterval: 5 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override port",
			env:  map[string]string{EnvPort: "2222"},
			want: Config{Host: "10.11.99.1", Port: 2222, User: "root", Password: "", SyncDir: "./documents", SyncInterval: 5 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override user",
			env:  map[string]string{EnvUser: "admin"},
			want: Config{Host: "10.11.99.1", Port: 22, User: "admin", Password: "", SyncDir: "./documents", SyncInterval: 5 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override password",
			env:  map[string]string{EnvPassword: "secret123"},
			want: Config{Host: "10.11.99.1", Port: 22, User: "root", Password: "secret123", SyncDir: "./documents", SyncInterval: 5 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override sync dir",
			env:  map[string]string{EnvSyncDir: "/home/user/docs"},
			want: Config{Host: "10.11.99.1", Port: 22, User: "root", Password: "", SyncDir: "/home/user/docs", SyncInterval: 5 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override sync interval",
			env:  map[string]string{EnvSyncInterval: "10m"},
			want: Config{Host: "10.11.99.1", Port: 22, User: "root", Password: "", SyncDir: "./documents", SyncInterval: 10 * time.Minute, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override sync interval seconds",
			env:  map[string]string{EnvSyncInterval: "30s"},
			want: Config{Host: "10.11.99.1", Port: 22, User: "root", Password: "", SyncDir: "./documents", SyncInterval: 30 * time.Second, ConnectionTimeout: 30 * time.Second},
		},
		{
			name: "override all",
			env: map[string]string{
				EnvHost:              "10.0.0.1",
				EnvPort:              "8022",
				EnvUser:              "remarker",
				EnvPassword:          "hunter2",
				EnvSyncDir:           "/data",
				EnvSyncInterval:      "1h",
				EnvConnectionTimeout: "60s",
			},
			want: Config{Host: "10.0.0.1", Port: 8022, User: "remarker", Password: "hunter2", SyncDir: "/data", SyncInterval: 1 * time.Hour, ConnectionTimeout: 60 * time.Second},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear all env vars first, then set only the ones in the test
			for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval, EnvConnectionTimeout} {
				t.Setenv(env, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got := Load()
			if got != tc.want {
				t.Errorf("Load() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_InvalidPortFallsBack(t *testing.T) {
	tests := []struct {
		name string
		port string
		want int
	}{
		{"non-numeric", "abc", 22},
		{"negative", "-1", 22},
		{"zero", "0", 0},
		{"float", "22.5", 22},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear all other env vars
			for _, env := range []string{EnvHost, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval} {
				t.Setenv(env, "")
			}
			t.Setenv(EnvPort, tc.port)

			c := Load()

			// For non-numeric and negative, strconv.Atoi returns error -> default 22
			// For "0", strconv.Atoi returns 0, no error -> port becomes 0
			if c.Port != tc.want {
				t.Errorf("Port = %d, want %d", c.Port, tc.want)
			}
			// All other defaults should be untouched
			if c.Host != "10.11.99.1" {
				t.Errorf("Host = %q, want default %q", c.Host, "10.11.99.1")
			}
		})
	}
}

func TestLoad_InvalidDurationFallsBack(t *testing.T) {
	tests := []struct {
		name string
		dur  string
		want time.Duration
	}{
		{"garbage text", "not-a-duration", 5 * time.Minute},
		{"empty string", "", 5 * time.Minute},
		{"valid duration", "15m", 15 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir} {
				t.Setenv(env, "")
			}
			t.Setenv(EnvSyncInterval, tc.dur)

			c := Load()

			if c.SyncInterval != tc.want {
				t.Errorf("SyncInterval = %v, want %v", c.SyncInterval, tc.want)
			}
		})
	}
}

func TestLoad_ConnectionTimeoutDefault(t *testing.T) {
	for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval, EnvConnectionTimeout} {
		t.Setenv(env, "")
	}

	c := Load()
	if c.ConnectionTimeout != 30*time.Second {
		t.Errorf("ConnectionTimeout = %v, want %v", c.ConnectionTimeout, 30*time.Second)
	}
}

func TestLoad_ConnectionTimeoutEnvOverride(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"custom 60s", "60s", 60 * time.Second},
		{"custom 1m", "1m", 1 * time.Minute},
		{"custom 5s", "5s", 5 * time.Second},
		{"invalid falls back", "not-a-duration", 30 * time.Second},
		{"empty falls back", "", 30 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval} {
				t.Setenv(env, "")
			}
			t.Setenv(EnvConnectionTimeout, tc.value)

			c := Load()
			if c.ConnectionTimeout != tc.want {
				t.Errorf("ConnectionTimeout = %v, want %v", c.ConnectionTimeout, tc.want)
			}
		})
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	c := Config{
		Host:         "10.11.99.1",
		Port:         22,
		User:         "root",
		Password:     "mysecret",
		SyncDir:      "./documents",
		SyncInterval: 5 * time.Minute,
	}

	if err := c.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_EmptyPassword(t *testing.T) {
	c := Config{
		Host:     "10.11.99.1",
		Port:     22,
		User:     "root",
		Password: "",
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for empty password")
	}
	if got := err.Error(); got != "REMARKABLE_PASSWORD environment variable is required" {
		t.Errorf("Validate() error = %q, want %q", got, "REMARKABLE_PASSWORD environment variable is required")
	}
}

func TestValidate_EmptyHost(t *testing.T) {
	c := Config{
		Host:     "",
		Port:     22,
		User:     "root",
		Password: "valid",
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for empty host")
	}
	if got := err.Error(); got != "host cannot be empty" {
		t.Errorf("Validate() error = %q, want %q", got, "host cannot be empty")
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port zero", 0},
		{"port negative", -1},
		{"port too high", 99999},
		{"port above max", 65536},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{
				Host:     "10.11.99.1",
				Port:     tc.port,
				User:     "root",
				Password: "valid",
			}

			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error for port %d", tc.port)
			}
			if got := err.Error(); got != "port must be between 1 and 65535, got "+string(rune(tc.port+'0')) {
				// Just verify it contains "port must be between"
				wantSubstr := "port must be between 1 and 65535"
				if len(got) < len(wantSubstr) || got[:len(wantSubstr)] != wantSubstr {
					t.Errorf("Validate() error = %q, want substring %q", got, wantSubstr)
				}
			}
		})
	}
}

func TestValidate_ValidBoundaryPorts(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port 1", 1},
		{"port 65535", 65535},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{
				Host:     "10.11.99.1",
				Port:     tc.port,
				User:     "root",
				Password: "valid",
			}

			if err := c.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil for port %d", err, tc.port)
			}
		})
	}
}

func TestLoad_FromEnvIntegration(t *testing.T) {
	// Simulate a realistic .env scenario
	t.Setenv(EnvHost, "10.11.99.1")
	t.Setenv(EnvPort, "22")
	t.Setenv(EnvUser, "root")
	t.Setenv(EnvPassword, "mypassword")
	t.Setenv(EnvSyncDir, "/home/user/rm-docs")
	t.Setenv(EnvSyncInterval, "30s")

	c := Load()

	if err := c.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if c.Host != "10.11.99.1" {
		t.Errorf("Host = %q, want %q", c.Host, "10.11.99.1")
	}
	if c.Password != "mypassword" {
		t.Errorf("Password = %q, want %q", c.Password, "mypassword")
	}
	if c.SyncInterval != 30*time.Second {
		t.Errorf("SyncInterval = %v, want %v", c.SyncInterval, 30*time.Second)
	}
}

func TestLoad_PasswordNotInEnv(t *testing.T) {
	// When password env var is not set, Validate should fail
	for _, env := range []string{EnvHost, EnvPort, EnvUser, EnvPassword, EnvSyncDir, EnvSyncInterval, EnvConnectionTimeout} {
		t.Setenv(env, "")
	}

	// Explicitly ensure password is unset
	os.Unsetenv(EnvPassword)

	c := Load()

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error when password is not set")
	}
}
