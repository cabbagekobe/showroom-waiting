package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Record records an m3u8 HLS stream to outPath using ffmpeg.
// When the broadcast ends and the HLS runs out, ffmpeg exits on its own.
// If ctx is canceled (Ctrl+C), ffmpeg is stopped and the recording captured so far is kept.
func Record(ctx context.Context, m3u8URL, outPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "warning",
		// Harden live HLS recording.
		// SHOWROOM's CDN behaves poorly with keepalive: reused connections return 404 for segments.
		// Disable persistent connections so each request uses a fresh connection (i.e. a new txspiseq token).
		"-http_persistent", "0",
		// Auto-reconnect on transient drops (also applied to non-seekable streaming input).
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

	if err := cmd.Run(); err != nil {
		// If ctx was canceled via Ctrl+C, treat it as a normal exit and keep the recorded file.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("ffmpeg execution error: %w", err)
	}
	return nil
}
