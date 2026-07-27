package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRoomKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"full URL", "https://www.showroom-live.com/nba_b_hinaho0303", "nba_b_hinaho0303", false},
		{"/r/ form", "https://www.showroom-live.com/r/fd4624681405", "fd4624681405", false},
		{"/r/ form + trailing slash", "https://www.showroom-live.com/r/fd4624681405/", "fd4624681405", false},
		{"/r/ form + query", "https://www.showroom-live.com/r/fd4624681405?foo=bar", "fd4624681405", false},
		{"trailing slash", "https://www.showroom-live.com/nba_b_hinaho0303/", "nba_b_hinaho0303", false},
		{"with query", "https://www.showroom-live.com/nba_b_hinaho0303?foo=bar", "nba_b_hinaho0303", false},
		{"http scheme", "http://www.showroom-live.com/some_key", "some_key", false},
		{"bare url_key", "nba_b_hinaho0303", "nba_b_hinaho0303", false},
		{"surrounding whitespace", "  nba_b_hinaho0303  ", "nba_b_hinaho0303", false},
		{"empty string", "", "", true},
		{"slash only", "https://www.showroom-live.com/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRoomKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRoomKey(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseRoomKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelectHLS(t *testing.T) {
	list := []StreamURL{
		{ID: 200, Type: "webrtc", Quality: 0, URL: "webrtc://a"},
		{ID: 2, Type: "hls", Quality: 1000, Label: "original quality", URL: "https://best.m3u8"},
		{ID: 6, Type: "hls", Quality: 200, Label: "Medium quality", URL: "https://medium.m3u8"},
		{ID: 4, Type: "hls", Quality: 100, Label: "low quality", URL: "https://low.m3u8"},
		{ID: 100, Type: "hls_all", Quality: 0, URL: "https://abr.m3u8"},
	}

	tests := []struct {
		name    string
		list    []StreamURL
		pref    string
		wantURL string
		wantOK  bool
	}{
		{"best", list, "best", "https://best.m3u8", true},
		{"medium", list, "medium", "https://medium.m3u8", true},
		{"low", list, "low", "https://low.m3u8", true},
		{"webrtc only is false", []StreamURL{{ID: 200, Type: "webrtc", URL: "webrtc://a"}}, "best", "", false},
		{"empty list is false", nil, "best", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SelectHLS(tt.list, tt.pref)
			if ok != tt.wantOK {
				t.Fatalf("SelectHLS ok=%v, want %v", ok, tt.wantOK)
			}
			if ok && got.URL != tt.wantURL {
				t.Errorf("SelectHLS url=%q, want %q", got.URL, tt.wantURL)
			}
		})
	}
}

func TestSelectHLS_MediumFallsBackToNearest(t *testing.T) {
	// When medium is requested but absent, fall back to the highest-quality hls.
	list := []StreamURL{
		{ID: 2, Type: "hls", Quality: 1000, URL: "https://best.m3u8"},
		{ID: 4, Type: "hls", Quality: 100, URL: "https://low.m3u8"},
	}
	got, ok := SelectHLS(list, "medium")
	if !ok {
		t.Fatal("SelectHLS should find an HLS stream")
	}
	if got.URL != "https://best.m3u8" {
		t.Errorf("fallback url=%q, want best", got.URL)
	}
}

func TestValidQuality(t *testing.T) {
	for _, q := range []string{"best", "medium", "low"} {
		if !validQuality(q) {
			t.Errorf("validQuality(%q) = false, want true", q)
		}
	}
	for _, q := range []string{"", "high", "BEST", "hoge"} {
		if validQuality(q) {
			t.Errorf("validQuality(%q) = true, want false", q)
		}
	}
}

func TestRecordingName(t *testing.T) {
	// Use StartedAt when it is set.
	if got := recordingName("akari", RoomStatus{StartedAt: 1784798269, LiveID: 999}); got != "akari_1784798269.ts" {
		t.Errorf("recordingName = %q, want akari_1784798269.ts", got)
	}
	// Fall back to LiveID when StartedAt is 0.
	if got := recordingName("akari", RoomStatus{StartedAt: 0, LiveID: 23198155}); got != "akari_23198155.ts" {
		t.Errorf("recordingName fallback = %q, want akari_23198155.ts", got)
	}
}

func TestResolveBaseDir(t *testing.T) {
	tests := []struct {
		name    string
		flagDir string
		cfgDir  string
		want    string
	}{
		{"flag takes priority", "/tmp/flag", "/tmp/cfg", "/tmp/flag"},
		{"falls back to config", "", "/tmp/cfg", "/tmp/cfg"},
		{"both empty defaults to current dir", "", "", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveBaseDir(tt.flagDir, tt.cfgDir); got != tt.want {
				t.Errorf("resolveBaseDir(%q, %q) = %q, want %q", tt.flagDir, tt.cfgDir, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home directory: %v", err)
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"expand ~/foo", "~/recordings", filepath.Join(home, "recordings")},
		{"expand bare ~", "~", home},
		{"path not starting with ~ is unchanged", "/tmp/rec", "/tmp/rec"},
		{"relative path is unchanged", "rec", "rec"},
		{"~ in the middle is not expanded", "/a/~/b", "/a/~/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandHome(tt.in); got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Isolate the test by pointing XDG_CONFIG_HOME at a temp directory.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// A missing file yields a zero value and no error.
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("a missing config file should not error: %v", err)
	}
	if cfg.OutputDir != "" {
		t.Errorf("OutputDir when unset = %q, want empty string", cfg.OutputDir)
	}

	// Once the config file exists, it is loaded.
	cfgDir := filepath.Join(dir, "showroom-waiting")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"output_dir": "~/recordings"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.OutputDir != "~/recordings" {
		t.Errorf("OutputDir = %q, want ~/recordings", cfg.OutputDir)
	}

	// An empty or whitespace-only file is treated as "no config": zero value and no error.
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("  \n\t"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("an empty file should not error: %v", err)
	}
	if cfg.OutputDir != "" {
		t.Errorf("OutputDir for an empty file = %q, want empty string", cfg.OutputDir)
	}

	// Invalid JSON is an error.
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Error("invalid JSON should error, but got nil")
	}
}

func TestFetchRoomStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("room_url_key") != "test_key" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"room_id": 108198, "is_live": true, "live_id": 23198155, "room_name": "Test Room",
		})
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), StatusURL: srv.URL}
	st, err := c.FetchRoomStatus(t.Context(), "test_key")
	if err != nil {
		t.Fatalf("FetchRoomStatus error: %v", err)
	}
	if st.RoomID != 108198 || !st.IsLive || st.RoomName != "Test Room" {
		t.Errorf("unexpected status: %+v", st)
	}

	if _, err := c.FetchRoomStatus(t.Context(), "unknown"); err == nil {
		t.Error("a nonexistent room should error")
	}
}

func TestFetchStreamingURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"streaming_url_list": []map[string]any{
				{"id": 2, "type": "hls", "quality": 1000, "is_default": false, "label": "original quality", "url": "https://best.m3u8"},
				{"id": 200, "type": "webrtc", "quality": 0, "is_default": true, "url": "webrtc://a"},
			},
		})
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), StreamURL: srv.URL}
	list, err := c.FetchStreamingURLs(t.Context(), 108198)
	if err != nil {
		t.Fatalf("FetchStreamingURLs error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d, want 2", len(list))
	}
	if got, ok := SelectHLS(list, "best"); !ok || got.URL != "https://best.m3u8" {
		t.Errorf("SelectHLS from fetched list failed: %+v ok=%v", got, ok)
	}
}
