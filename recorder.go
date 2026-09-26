package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Record records an m3u8 HLS stream to outPath using ffmpeg.
// When the broadcast ends and the HLS runs out, ffmpeg exits on its own.
// If ctx is canceled (Ctrl+C), ffmpeg is stopped and the recording captured so far is kept.
// A verbose ffmpeg log is written next to the recording (outPath + ".log") so segment-level
// problems (fetch failures, skipped sequence numbers) can be diagnosed after the fact.
func Record(ctx context.Context, m3u8URL, outPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "warning",
		// Harden live HLS recording.
		// Use a fresh connection per request. Measured on 2026-09-25: connection reuse neither causes nor avoids
		// the CDN's 404s on freshly listed segments (see -seg_max_retry below), so this is kept only as a
		// conservative default.
		"-http_persistent", "0",
		// Retry a segment on fetch failure instead of skipping it (the HLS demuxer's default is 0 = skip).
		// The CDN returns 404 for a freshly listed segment for ~1s before it becomes fetchable. ffmpeg retries
		// back-to-back (~0.1s per attempt), and up to 9 attempts were needed in practice, so allow plenty.
		// This cannot stall forever: once the segment leaves the ~3-segment live window, ffmpeg gives up on it.
		"-seg_max_retry", "30",
		// Auto-reconnect on transient drops. Note: these only apply to the initial playlist fetch;
		// the HLS demuxer does not forward reconnect options to segment requests.
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", m3u8URL,
		"-c", "copy",
		"-f", "mpegts",
		outPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Keep the terminal quiet (warning level) but write a verbose (level 40) report file next to the recording.
	cmd.Env = ffreportEnv(os.Environ(), outPath)

	if err := cmd.Run(); err != nil {
		// If ctx was canceled via Ctrl+C, treat it as a normal exit and keep the recorded file.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("ffmpeg execution error: %w", err)
	}
	return nil
}

// ffreportEnv is a pure function that returns env with FFREPORT set to write a verbose report next to outPath.
// Any FFREPORT already present is dropped first: with duplicate entries, getenv returns the earlier one,
// which would silently disable the recording's log file.
func ffreportEnv(env []string, outPath string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "FFREPORT=") {
			out = append(out, kv)
		}
	}
	return append(out, "FFREPORT=file="+ffreportFile(outPath)+":level=40")
}

// ffreportFile is a pure function that builds the FFREPORT "file=" value for outPath (outPath + ".log").
// ffmpeg parses the value with ':' as the option separator, backslash as escape and single quotes as quoting,
// then expands '%' templates, so those characters in the path must be escaped.
func ffreportFile(outPath string) string {
	return strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\'`, `%`, `%%`).Replace(outPath + ".log")
}
