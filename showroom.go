package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// errNotFound signals that the API returned 404 (the room does not exist).
var errNotFound = errors.New("not found")

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) showroom-waiting"

// Client talks to SHOWROOM's public API.
// Endpoints are kept as fields so they can be swapped out in tests.
type Client struct {
	HTTP      *http.Client
	StatusURL string // default: https://www.showroom-live.com/api/room/status
	StreamURL string // default: https://www.showroom-live.com/api/live/streaming_url
}

// NewClient builds a Client pointed at the production endpoints.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 15 * time.Second},
		StatusURL: "https://www.showroom-live.com/api/room/status",
		StreamURL: "https://www.showroom-live.com/api/live/streaming_url",
	}
}

// RoomStatus holds the fields we use from /api/room/status.
type RoomStatus struct {
	RoomID    int    `json:"room_id"`
	IsLive    bool   `json:"is_live"`
	LiveID    int    `json:"live_id"`
	RoomName  string `json:"room_name"`
	StartedAt int64  `json:"started_at"`
}

// StreamURL is a single entry of streaming_url_list.
type StreamURL struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	Quality   int    `json:"quality"`
	IsDefault bool   `json:"is_default"`
	Label     string `json:"label"`
}

// ParseRoomKey is a pure function that extracts room_url_key from the input (a room URL or url_key).
func ParseRoomKey(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("URL or url_key is empty")
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("cannot parse URL: %w", err)
		}
		// The room key is the last path segment; handle both `/room_key` and `/r/room_key`.
		segs := strings.Split(strings.Trim(u.Path, "/"), "/")
		key := segs[len(segs)-1]
		if key == "" {
			return "", fmt.Errorf("cannot determine room key from URL: %s", s)
		}
		return key, nil
	}
	return s, nil
}

// SelectHLS is a pure function that picks one ffmpeg-recordable HLS stream from streaming_url_list.
// pref is "best"/"medium"/"low"; if the requested quality is absent it falls back to the highest-quality HLS.
// If there is no HLS stream at all, ok is false.
func SelectHLS(list []StreamURL, pref string) (StreamURL, bool) {
	var hls []StreamURL
	for _, s := range list {
		if s.Type == "hls" {
			hls = append(hls, s)
		}
	}
	if len(hls) == 0 {
		return StreamURL{}, false
	}

	best := hls[0]
	for _, s := range hls[1:] {
		if s.Quality > best.Quality {
			best = s
		}
	}

	var wantQuality int
	switch pref {
	case "low":
		wantQuality = 100
	case "medium":
		wantQuality = 200
	default: // best
		return best, true
	}
	for _, s := range hls {
		if s.Quality == wantQuality {
			return s, true
		}
	}
	return best, true // fall back to the highest quality when the requested one is absent
}

func (c *Client) getJSON(ctx context.Context, endpoint string, q url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d (%s)", resp.StatusCode, endpoint)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// FetchRoomStatus fetches the live status for a room_url_key. A nonexistent room is an error.
func (c *Client) FetchRoomStatus(ctx context.Context, urlKey string) (RoomStatus, error) {
	var st RoomStatus
	q := url.Values{"room_url_key": {urlKey}}
	if err := c.getJSON(ctx, c.StatusURL, q, &st); err != nil {
		if errors.Is(err, errNotFound) {
			return RoomStatus{}, fmt.Errorf("room not found: %s", urlKey)
		}
		return RoomStatus{}, err
	}
	if st.RoomID == 0 {
		return RoomStatus{}, fmt.Errorf("room not found: %s", urlKey)
	}
	return st, nil
}

// FetchStreamingURLs fetches the list of streaming URLs for a live room.
func (c *Client) FetchStreamingURLs(ctx context.Context, roomID int) ([]StreamURL, error) {
	var out struct {
		List []StreamURL `json:"streaming_url_list"`
	}
	q := url.Values{"room_id": {fmt.Sprint(roomID)}, "abr_available": {"1"}}
	if err := c.getJSON(ctx, c.StreamURL, q, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}
