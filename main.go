// Package main implements showroom-waiting, a CLI that waits for a SHOWROOM
// room to start broadcasting and then records the HLS stream.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	outDir := flag.String("o", "", "base directory for recordings (falls back to the config file, then the current directory)")
	quality := flag.String("q", "best", "quality best|medium|low")
	interval := flag.Int("i", 30, "polling interval in seconds for checking whether the broadcast has started")
	timeoutMin := flag.Int("timeout", 0, "maximum wait time in minutes; 0 means unlimited")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: showroom-waiting <room URL or url_key> [options]")
		fmt.Fprintln(os.Stderr, "\nWaits until the specified SHOWROOM room starts broadcasting, then records it.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
	}
	// Allow the positional argument (URL/url_key) to appear before or after flags.
	// Go's standard flag stops parsing at the first non-flag argument, so if the first arg is positional, strip it before parsing.
	roomArg := ""
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		roomArg = args[0]
		args = args[1:]
	}
	// flag.CommandLine uses ExitOnError, so Parse never returns a non-nil error here.
	_ = flag.CommandLine.Parse(args)
	if roomArg == "" {
		if flag.NArg() < 1 {
			flag.Usage()
			return fmt.Errorf("please specify a room URL or url_key")
		}
		roomArg = flag.Arg(0)
	}

	if *interval < 1 {
		return fmt.Errorf("-i (polling interval) must be at least 1 second: %d", *interval)
	}
	if !validQuality(*quality) {
		return fmt.Errorf("-q (quality) must be one of best / medium / low: %q", *quality)
	}

	urlKey, err := ParseRoomKey(roomArg)
	if err != nil {
		return err
	}

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	// Record into a url_key subdirectory created beneath the base directory.
	baseDir := expandHome(resolveBaseDir(*outDir, cfg.OutputDir))
	roomDir := filepath.Join(baseDir, urlKey)

	// Let Ctrl+C stop both waiting and recording.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := NewClient()

	// Verify the room exists before starting to wait.
	st, err := client.FetchRoomStatus(ctx, urlKey)
	if err != nil {
		return err
	}
	fmt.Printf("target room: %s (room_id=%d, url_key=%s)\n", st.RoomName, st.RoomID, urlKey)

	// Prepare the output directory before waiting, so a long wait doesn't fail later on a bad output path.
	if err := os.MkdirAll(roomDir, 0o755); err != nil {
		return fmt.Errorf("cannot prepare output directory: %w", err)
	}

	st, err = waitUntilLive(ctx, client, urlKey, st, *interval, *timeoutMin)
	if err != nil {
		return err
	}

	fmt.Println("broadcast start detected. Fetching the streaming URL…")
	stream, err := resolveHLS(ctx, client, st.RoomID, *quality)
	if err != nil {
		return err
	}

	outPath := filepath.Join(roomDir, recordingName(urlKey, st))
	fmt.Printf("starting recording: %s\nquality: %s\noutput: %s\n", stream.URL, stream.Label, outPath)

	if err := Record(ctx, stream.URL, outPath); err != nil {
		return err
	}

	fmt.Printf("recording finished: %s\n", outPath)
	return nil
}

// waitUntilLive polls until is_live becomes true. If already live, it returns immediately.
func waitUntilLive(ctx context.Context, c *Client, urlKey string, st RoomStatus, intervalSec, timeoutMin int) (RoomStatus, error) {
	if st.IsLive {
		return st, nil
	}

	var deadline time.Time
	if timeoutMin > 0 {
		deadline = time.Now().Add(time.Duration(timeoutMin) * time.Minute)
	}

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	fmt.Printf("waiting to record… checking for broadcast start every %d seconds (Ctrl+C to cancel)\n", intervalSec)
	for {
		fmt.Printf("[%s] waiting: %s is not broadcasting yet\n", time.Now().Format("15:04:05"), urlKey)
		if !deadline.IsZero() && time.Now().After(deadline) {
			return RoomStatus{}, fmt.Errorf("exceeded the maximum wait time (%d minutes)", timeoutMin)
		}

		select {
		case <-ctx.Done():
			return RoomStatus{}, fmt.Errorf("waiting canceled")
		case <-ticker.C:
		}

		next, err := c.FetchRoomStatus(ctx, urlKey)
		if err != nil {
			// Don't stop waiting on a transient fetch failure; retry on the next cycle.
			fmt.Fprintf(os.Stderr, "warning: failed to fetch status (continuing): %v\n", err)
			continue
		}
		if next.IsLive {
			return next, nil
		}
	}
}

// resolveHLS retries streaming_url a few times (it can come back empty right after a broadcast starts) to settle on an HLS stream.
func resolveHLS(ctx context.Context, c *Client, roomID int, quality string) (StreamURL, error) {
	const maxRetry = 10
	for i := 0; i < maxRetry; i++ {
		list, err := c.FetchStreamingURLs(ctx, roomID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch streaming URL (retrying): %v\n", err)
		} else if s, ok := SelectHLS(list, quality); ok {
			return s, nil
		}
		select {
		case <-ctx.Done():
			return StreamURL{}, fmt.Errorf("canceled")
		case <-time.After(3 * time.Second):
		}
	}
	return StreamURL{}, fmt.Errorf("could not obtain a recordable HLS stream")
}

// validQuality reports whether q is an accepted value for -q.
func validQuality(q string) bool {
	switch q {
	case "best", "medium", "low":
		return true
	}
	return false
}

// recordingName builds a collision-resistant output file name.
func recordingName(urlKey string, st RoomStatus) string {
	stamp := st.StartedAt
	if stamp == 0 {
		stamp = int64(st.LiveID)
	}
	return fmt.Sprintf("%s_%d.ts", urlKey, stamp)
}
