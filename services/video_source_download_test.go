package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fastDownloadPolicy removes real backoff and disk probing for tests and
// restores everything at the end.
func fastDownloadPolicy(t *testing.T) {
	t.Helper()
	prevBackoff, prevIdle, prevDisk := videoDownloadBackoff, videoDownloadIdleTimeout, availableDiskSpace
	videoDownloadBackoff = []time.Duration{0, 0, 0}
	availableDiskSpace = func(string) (uint64, error) { return 1 << 40, nil }
	t.Cleanup(func() {
		videoDownloadBackoff, videoDownloadIdleTimeout, availableDiskSpace = prevBackoff, prevIdle, prevDisk
	})
}

func testVideoBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*31 + i/256)
	}
	return b
}

// sourceObject simulates stored R2 media. It serves Range requests like real
// storage. It can also cut the first GET short and reset the connection.
type sourceObject struct {
	content  []byte
	cutAfter int // bytes sent on the first plain GET before the connection is reset
	stall    bool
	gets     int32
	mu       sync.Mutex
	ranges   []string
	status   int // when non-zero, every GET returns this status
}

func (o *sourceObject) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		n := atomic.AddInt32(&o.gets, 1)
		o.mu.Lock()
		o.ranges = append(o.ranges, r.Header.Get("Range"))
		o.mu.Unlock()

		if o.status != 0 {
			http.Error(w, "nope", o.status)
			return
		}
		if n == 1 && r.Header.Get("Range") == "" && o.cutAfter > 0 {
			w.Header().Set("Content-Length", strconv.Itoa(len(o.content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(o.content[:o.cutAfter])
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close() // connection reset mid-body
			}
			return
		}
		if o.stall {
			// Send a few bytes, then go silent until the client gives up.
			w.Header().Set("Content-Length", strconv.Itoa(len(o.content)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(o.content[:10])
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
	}
	http.ServeContent(w, r, "match.mp4", time.Time{}, bytes.NewReader(o.content))
}

func (o *sourceObject) rangeHeaders() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.ranges...)
}

func TestDownloadSourceVideoStreamsWholeObject(t *testing.T) {
	fastDownloadPolicy(t)
	content := testVideoBytes(2 << 20)
	obj := &sourceObject{content: content}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	n, err := downloadSourceVideo(context.Background(), srv.URL+"/match.mp4", dst)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if n != int64(len(content)) {
		t.Fatalf("wrote %d bytes, want %d", n, len(content))
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, content) {
		t.Fatalf("downloaded bytes differ from source")
	}
	if atomic.LoadInt32(&obj.gets) != 1 {
		t.Fatalf("expected a single GET, got %d", obj.gets)
	}
}

func TestDownloadResumesAfterConnectionReset(t *testing.T) {
	fastDownloadPolicy(t)
	content := testVideoBytes(3 << 20)
	cut := 1 << 20
	obj := &sourceObject{content: content, cutAfter: cut}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	if _, err := downloadSourceVideo(context.Background(), srv.URL+"/match.mp4", dst); err != nil {
		t.Fatalf("expected recovery after reset, got: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, content) {
		t.Fatalf("resumed file is corrupt: got %d bytes", len(got))
	}
	ranges := obj.rangeHeaders()
	if len(ranges) != 2 || ranges[1] != "bytes="+strconv.Itoa(cut)+"-" {
		t.Fatalf("expected resume from byte %d, got ranges %q", cut, ranges)
	}
}

func TestDownloadDoesNotRetryPermanentStorageError(t *testing.T) {
	fastDownloadPolicy(t)
	obj := &sourceObject{content: testVideoBytes(1024), status: http.StatusNotFound}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	_, err := downloadSourceVideo(context.Background(), srv.URL+"/gone.mp4", dst)

	var srcErr *VideoSourceError
	if !errors.As(err, &srcErr) || srcErr.Code != videoSourceCodeStorageUnavailable {
		t.Fatalf("want storage-unavailable error, got %v", err)
	}
	if srcErr.Message != "Video was uploaded but could not be retrieved from storage." {
		t.Fatalf("unexpected message: %q", srcErr.Message)
	}
	if n := atomic.LoadInt32(&obj.gets); n != 1 {
		t.Fatalf("404 must not be retried, got %d GETs", n)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("partial file must be removed after failure")
	}
}

func TestDownloadGivesUpAfterMaxAttempts(t *testing.T) {
	fastDownloadPolicy(t)
	obj := &sourceObject{content: testVideoBytes(1024), status: http.StatusInternalServerError}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	_, err := downloadSourceVideo(context.Background(), srv.URL+"/match.mp4", dst)

	var srcErr *VideoSourceError
	if !errors.As(err, &srcErr) || srcErr.Code != videoSourceCodeDownloadInterrupted {
		t.Fatalf("want interrupted error, got %v", err)
	}
	if srcErr.Message != "Video download interrupted while retrieving source video. Retry the processing job." {
		t.Fatalf("unexpected message: %q", srcErr.Message)
	}
	if n := atomic.LoadInt32(&obj.gets); n != int32(videoDownloadMaxAttempts) {
		t.Fatalf("want %d attempts, got %d", videoDownloadMaxAttempts, n)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("partial file must be removed after final failure")
	}
}

func TestDownloadIdleTimeoutIsRetriedThenFails(t *testing.T) {
	fastDownloadPolicy(t)
	videoDownloadIdleTimeout = 150 * time.Millisecond
	obj := &sourceObject{content: testVideoBytes(1 << 20), stall: true}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	_, err := downloadSourceVideo(context.Background(), srv.URL+"/match.mp4", dst)

	var srcErr *VideoSourceError
	if !errors.As(err, &srcErr) || srcErr.Code != videoSourceCodeDownloadInterrupted {
		t.Fatalf("stalled body should end as interrupted, got %v", err)
	}
	if n := atomic.LoadInt32(&obj.gets); n != int32(videoDownloadMaxAttempts) {
		t.Fatalf("stalled body should be retried to the limit, got %d GETs", n)
	}
}

func TestDownloadRejectsWhenTempDiskIsTooSmall(t *testing.T) {
	fastDownloadPolicy(t)
	availableDiskSpace = func(string) (uint64, error) { return 1024, nil }
	obj := &sourceObject{content: testVideoBytes(4096)}
	srv := httptest.NewServer(obj)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "source.mp4")
	_, err := downloadSourceVideo(context.Background(), srv.URL+"/match.mp4", dst)

	var srcErr *VideoSourceError
	if !errors.As(err, &srcErr) || srcErr.Code != videoSourceCodeInsufficientStorage {
		t.Fatalf("want insufficient-storage error, got %v", err)
	}
	if srcErr.Message != "Insufficient temporary storage for this video." {
		t.Fatalf("unexpected message: %q", srcErr.Message)
	}
	if n := atomic.LoadInt32(&obj.gets); n != 0 {
		t.Fatalf("no GET should start when disk is short, got %d", n)
	}
}

func TestEnsureTempStorageForRequiresSourceSizeHeadroom(t *testing.T) {
	const gib = int64(1 << 30)
	availableDiskSpace = func(string) (uint64, error) { return uint64(3 * gib), nil }
	t.Cleanup(func() { availableDiskSpace = availableDiskBytes })

	if err := ensureTempStorageFor(t.TempDir(), 1*gib); err != nil {
		t.Fatalf("1 GiB source with 3 GiB free should pass: %v", err)
	}
	if err := ensureTempStorageFor(t.TempDir(), 2*gib); err == nil {
		t.Fatalf("2 GiB source needs ~4.5 GiB free and must be refused")
	}
}

func TestParseContentRangeTotal(t *testing.T) {
	for _, tc := range []struct {
		name      string
		header    string
		offset    int64
		remaining int64
		want      int64
	}{
		{"uses total from header", "bytes 100-199/1000", 100, 900, 1000},
		{"falls back to offset plus remaining", "", 100, 900, 1000},
		{"unknown when nothing usable", "", 100, -1, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseContentRangeTotal(tc.header, tc.offset, tc.remaining); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDescribeVideoSourceDropsQueryAndHost(t *testing.T) {
	got := describeVideoSource("https://pub-x.r2.dev/matches/9/123-match.mp4?X-Amz-Signature=secret")
	if got != "/matches/9/123-match.mp4" {
		t.Fatalf("log label leaked or wrong: %q", got)
	}
}

func TestSourceVideoErrorIsSafeToShow(t *testing.T) {
	err := classifyDownloadFailure(downloadFailureRetryable, errors.New("read tcp 10.0.0.1: connection reset by peer"))
	if err.Message == err.Detail {
		t.Fatalf("user message must not be the raw transport error")
	}
	if err.Detail == "" {
		t.Fatalf("technical detail should be kept for diagnostics")
	}
}

func TestParseFFprobeOutput(t *testing.T) {
	valid := []byte(`{
		"streams": [
			{"codec_type":"audio","codec_name":"aac"},
			{"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"25/1","r_frame_rate":"25/1"}
		],
		"format": {"duration":"5400.250000","bit_rate":"3500000","size":"2362500000"}
	}`)
	info, err := parseFFprobeOutput(valid)
	if err != nil {
		t.Fatalf("valid 90 minute probe rejected: %v", err)
	}
	if info.DurationSec != 5400.25 || info.Width != 1920 || info.Height != 1080 || info.FrameRate != 25 ||
		info.Codec != "h264" || info.BitRate != 3500000 || info.SizeBytes != 2362500000 {
		t.Fatalf("unexpected probe info: %+v", info)
	}

	for name, raw := range map[string][]byte{
		"no video stream": []byte(`{"streams":[{"codec_type":"audio"}],"format":{"duration":"10"}}`),
		"zero duration":   []byte(`{"streams":[{"codec_type":"video"}],"format":{"duration":"0"}}`),
		"not json":        []byte(`garbage`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFFprobeOutput(raw); !errors.Is(err, errInvalidVideo) {
				t.Fatalf("want errInvalidVideo, got %v", err)
			}
		})
	}
}

func TestParseFrameRate(t *testing.T) {
	if got := parseFrameRate("30000/1001"); got < 29.97 || got > 29.98 {
		t.Fatalf("NTSC rate parsed as %v", got)
	}
	if got := parseFrameRate("0/0"); got != 0 {
		t.Fatalf("0/0 should be unknown, got %v", got)
	}
	if got := parseFrameRate("25"); got != 0 {
		t.Fatalf("missing denominator should be unknown, got %v", got)
	}
}

func TestProbeVideoFileWithFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i",
		"testsrc=duration=2:size=64x48:rate=10", "-pix_fmt", "yuv420p", good).CombinedOutput(); err != nil {
		t.Fatalf("could not generate test clip: %v: %s", err, out)
	}

	info, err := probeVideoFile(context.Background(), good)
	if err != nil {
		t.Fatalf("probe failed on valid clip: %v", err)
	}
	if info.Width != 64 || info.Height != 48 || info.DurationSec < 1.5 || info.DurationSec > 2.5 {
		t.Fatalf("unexpected probe: %+v", info)
	}

	bad := filepath.Join(dir, "bad.mp4")
	if err := os.WriteFile(bad, []byte("this is not a video file at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := probeVideoFile(context.Background(), bad); !errors.Is(err, errInvalidVideo) {
		t.Fatalf("corrupt file should be rejected as invalid, got %v", err)
	}
}
