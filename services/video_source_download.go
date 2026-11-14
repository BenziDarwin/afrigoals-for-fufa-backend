// services/video_source_download.go
package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Source video download policy.
//
// The stored match video is streamed straight to a temp file. Nothing is held
// in RAM. A dropped connection is resumed with an HTTP Range request from the
// bytes already on disk, and only idempotent GETs are retried.
const (
	videoDownloadMaxAttempts    = 3
	videoDownloadOverallTimeout = 3 * time.Hour // hard cap per attempt
	videoDownloadHeadTimeout    = 30 * time.Second
)

// Delay before attempt 2, 3, ... (2s, 5s, 10s policy).
var videoDownloadBackoff = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// Overridable in tests.
var (
	videoDownloadHTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 2 * time.Minute,
			// Large transfers should not reuse a connection that a proxy may have
			// silently reset. Each attempt opens a fresh connection.
			DisableKeepAlives: true,
		},
	}
	videoDownloadIdleTimeout = 2 * time.Minute // max silence between body reads
	availableDiskSpace       = availableDiskBytes
)

const (
	videoSourceCodeDownloadInterrupted = "VIDEO_DOWNLOAD_INTERRUPTED"
	videoSourceCodeStorageUnavailable  = "VIDEO_STORAGE_UNAVAILABLE"
	videoSourceCodeInsufficientStorage = "INSUFFICIENT_TEMP_STORAGE"
)

// VideoSourceError is a user-facing failure while retrieving the source video.
// Message is safe to show to the analyst; Detail carries the technical cause.
type VideoSourceError struct {
	Code    string
	Message string
	Detail  string
}

func (e *VideoSourceError) Error() string {
	return e.Message
}

type downloadFailureKind int

const (
	downloadFailureRetryable downloadFailureKind = iota // network drop, timeout, 5xx, 429
	downloadFailurePermanent                            // storage said no (404, 403, ...)
	downloadFailureLocalDisk                            // could not write to our temp disk
)

// downloadSourceVideo copies sourceURL into dst. It returns the number of bytes
// written. On failure dst is removed so no partial file is left behind.
func downloadSourceVideo(ctx context.Context, sourceURL string, dst string) (int64, error) {
	source := describeVideoSource(sourceURL)
	expected := probeRemoteVideoSize(ctx, sourceURL)

	if err := ensureTempStorageFor(filepath.Dir(dst), expected); err != nil {
		log.Printf("VIDEO_DOWNLOAD_FAILED reason=insufficient_storage source=%s expected_bytes=%d err=%v", source, expected, err)
		return 0, err
	}

	log.Printf("VIDEO_DOWNLOAD_STARTED source=%s expected_bytes=%d max_attempts=%d", source, expected, videoDownloadMaxAttempts)

	var lastErr error
	var lastKind downloadFailureKind
	for attempt := 1; attempt <= videoDownloadMaxAttempts; attempt++ {
		total, kind, err := downloadSourceAttempt(ctx, sourceURL, dst, fileSizeOrZero(dst))
		if total > 0 {
			expected = total
		}
		if err == nil {
			written := fileSizeOrZero(dst)
			log.Printf("VIDEO_DOWNLOAD_COMPLETED source=%s bytes=%d attempt=%d", source, written, attempt)
			return written, nil
		}

		lastErr, lastKind = err, kind
		if kind != downloadFailureRetryable || ctx.Err() != nil || attempt == videoDownloadMaxAttempts {
			break
		}

		delay := videoDownloadBackoff[len(videoDownloadBackoff)-1]
		if attempt-1 < len(videoDownloadBackoff) {
			delay = videoDownloadBackoff[attempt-1]
		}
		log.Printf("VIDEO_DOWNLOAD_RETRY source=%s attempt=%d next_attempt_in=%s resume_from_bytes=%d err=%v",
			source, attempt, delay, fileSizeOrZero(dst), err)

		select {
		case <-ctx.Done():
		case <-time.After(delay):
		}
		if ctx.Err() != nil {
			break
		}
	}

	_ = os.Remove(dst)

	srcErr := classifyDownloadFailure(lastKind, lastErr)
	log.Printf("VIDEO_DOWNLOAD_FAILED source=%s code=%s expected_bytes=%d err=%v", source, srcErr.Code, expected, lastErr)
	return 0, srcErr
}

// downloadSourceAttempt performs one GET. When offset > 0 it asks for the
// remaining bytes and appends them. If the server ignores Range and replies
// 200, the file is truncated and the download restarts from zero.
func downloadSourceAttempt(ctx context.Context, sourceURL string, dst string, offset int64) (int64, downloadFailureKind, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, videoDownloadOverallTimeout)
	defer cancel()

	// Cancel the attempt if no bytes arrive for too long.
	idle := newIdleWatchdog(videoDownloadIdleTimeout, cancel)
	defer idle.stop()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return -1, downloadFailurePermanent, err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := videoDownloadHTTPClient.Do(req)
	if err != nil {
		return -1, downloadFailureRetryable, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	resumed := false
	total := resp.ContentLength
	switch {
	case resp.StatusCode == http.StatusPartialContent && offset > 0:
		resumed = true
		total = parseContentRangeTotal(resp.Header.Get("Content-Range"), offset, resp.ContentLength)
	case resp.StatusCode == http.StatusOK:
		offset = 0 // full body: truncate any partial file below
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0:
		// The partial file no longer matches the object. Start over.
		_ = os.Remove(dst)
		return -1, downloadFailureRetryable, fmt.Errorf("storage rejected resume at byte %d", offset)
	default:
		kind := downloadFailurePermanent
		if isRetryableHTTPStatus(resp.StatusCode) {
			kind = downloadFailureRetryable
		}
		return -1, kind, fmt.Errorf("storage returned HTTP %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resumed {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dst, flags, 0o644)
	if err != nil {
		return total, downloadFailureLocalDisk, err
	}

	writer := &diskWriter{f: f}
	written, copyErr := io.Copy(writer, idleReader{r: resp.Body, w: idle})
	closeErr := f.Close()

	have := written
	if resumed {
		have += offset
	}

	if closeErr != nil {
		return total, downloadFailureLocalDisk, closeErr
	}
	if copyErr != nil {
		kind := downloadFailureRetryable
		if writer.failed {
			kind = downloadFailureLocalDisk
		}
		return total, kind, fmt.Errorf("read interrupted after %d bytes: %w", have, copyErr)
	}
	if total > 0 && have < total {
		// Clean EOF before the advertised length. Treat it as a dropped connection.
		return total, downloadFailureRetryable, fmt.Errorf("connection closed after %d of %d bytes", have, total)
	}
	return total, downloadFailureRetryable, nil
}

func classifyDownloadFailure(kind downloadFailureKind, err error) *VideoSourceError {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	switch kind {
	case downloadFailureLocalDisk:
		return &VideoSourceError{
			Code:    videoSourceCodeInsufficientStorage,
			Message: "Insufficient temporary storage for this video.",
			Detail:  detail,
		}
	case downloadFailurePermanent:
		return &VideoSourceError{
			Code:    videoSourceCodeStorageUnavailable,
			Message: "Video was uploaded but could not be retrieved from storage.",
			Detail:  detail,
		}
	default:
		return &VideoSourceError{
			Code:    videoSourceCodeDownloadInterrupted,
			Message: "Video download interrupted while retrieving source video. Retry the processing job.",
			Detail:  detail,
		}
	}
}

// ensureTempStorageFor refuses to start a download that would fill the disk.
// It needs the source size plus room for a masked copy and some headroom.
// If free space cannot be measured, it logs and continues.
func ensureTempStorageFor(dir string, expectedBytes int64) error {
	const headroom = 512 << 20
	required := int64(1 << 30)
	if expectedBytes > 0 {
		required = expectedBytes*2 + headroom
	}

	free, err := availableDiskSpace(dir)
	if err != nil {
		log.Printf("VIDEO_DOWNLOAD_WARNING disk_check_unavailable dir=%s err=%v", dir, err)
		return nil
	}
	if free < uint64(required) {
		return &VideoSourceError{
			Code:    videoSourceCodeInsufficientStorage,
			Message: "Insufficient temporary storage for this video.",
			Detail:  fmt.Sprintf("need about %d bytes free, have %d", required, free),
		}
	}
	return nil
}

// probeRemoteVideoSize asks for the object size with HEAD. A failure returns -1
// and the download still runs.
func probeRemoteVideoSize(ctx context.Context, sourceURL string) int64 {
	reqCtx, cancel := context.WithTimeout(ctx, videoDownloadHeadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, sourceURL, nil)
	if err != nil {
		return -1
	}
	resp, err := videoDownloadHTTPClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		return -1
	}
	return resp.ContentLength
}

func isRetryableHTTPStatus(code int) bool {
	return code == http.StatusRequestTimeout ||
		code == http.StatusTooEarly ||
		code == http.StatusTooManyRequests ||
		code >= 500
}

// parseContentRangeTotal reads the full object size from "bytes a-b/total".
// Without a usable header it falls back to offset + the bytes still to come.
func parseContentRangeTotal(header string, offset int64, remaining int64) int64 {
	if idx := strings.LastIndex(header, "/"); idx >= 0 {
		if total, err := strconv.ParseInt(strings.TrimSpace(header[idx+1:]), 10, 64); err == nil && total > 0 {
			return total
		}
	}
	if remaining >= 0 {
		return offset + remaining
	}
	return -1
}

// describeVideoSource returns a log-safe label: the object path without the
// host or query string.
func describeVideoSource(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil || u.Path == "" {
		return "unknown"
	}
	return u.Path
}

func fileSizeOrZero(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// diskWriter records whether a write failed. A local write failure is not
// retryable. A network read failure is.
type diskWriter struct {
	f      *os.File
	failed bool
}

func (w *diskWriter) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	if err != nil {
		w.failed = true
	}
	return n, err
}

// idleWatchdog cancels an attempt when no bytes arrive for too long.
type idleWatchdog struct {
	timer   *time.Timer
	timeout time.Duration
}

func newIdleWatchdog(timeout time.Duration, onIdle func()) *idleWatchdog {
	return &idleWatchdog{timer: time.AfterFunc(timeout, onIdle), timeout: timeout}
}

func (w *idleWatchdog) touch() {
	w.timer.Reset(w.timeout)
}

func (w *idleWatchdog) stop() {
	w.timer.Stop()
}

type idleReader struct {
	r io.Reader
	w *idleWatchdog
}

func (r idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.w.touch()
	return n, err
}
