package services

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestModelUploadTimeoutBounds(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 60 * time.Minute},
		{"5400", 5400 * time.Second},
		{"30", 60 * time.Minute}, // below floor
		{"999999", 60 * time.Minute},
		{"junk", 60 * time.Minute},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("MODEL_SERVER_UPLOAD_TIMEOUT_SECONDS", tc.value)
			if got := modelUploadTimeout(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestUploadLocalVideoToModelStreamsMultipartWithCommentary(t *testing.T) {
	video := testVideoBytes(1 << 20)
	audio := []byte("ID3-fake-commentary")
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "match.mp4")
	audioPath := filepath.Join(dir, "commentary.mp3")
	if err := os.WriteFile(videoPath, video, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(audioPath, audio, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep parts on disk so the server side does not buffer them in memory either.
		if err := r.ParseMultipartForm(64 << 10); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		got, _ := io.ReadAll(f)
		if !bytes.Equal(got, video) {
			http.Error(w, "video bytes differ", http.StatusBadRequest)
			return
		}
		if r.FormValue("match_id") != "7" || r.FormValue("requested_by_user_id") != "3" {
			http.Error(w, "missing ids", http.StatusBadRequest)
			return
		}
		ca, _, err := r.FormFile("commentary_audio")
		if err != nil {
			http.Error(w, "missing commentary audio", http.StatusBadRequest)
			return
		}
		caBytes, _ := io.ReadAll(ca)
		if !bytes.Equal(caBytes, audio) || r.FormValue("commentary_audio_format") != "mp3" {
			http.Error(w, "commentary mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"model-123","status":"queued","message":"ok"}`))
	}))
	defer srv.Close()

	t.Setenv("MODEL_SERVER_UPLOAD_URL", srv.URL)
	up, status, body, err := uploadLocalVideoToModel(7, 3, "match.mp4", videoPath, modelUploadExtras{CommentaryAudioPath: audioPath})
	if err != nil {
		t.Fatalf("upload failed: status=%d body=%s err=%v", status, body, err)
	}
	if up.JobID != "model-123" || status != http.StatusOK {
		t.Fatalf("unexpected response: %+v status=%d", up, status)
	}
}

func TestUploadLocalVideoToModelReportsNon2xxWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "model busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	videoPath := filepath.Join(t.TempDir(), "match.mp4")
	if err := os.WriteFile(videoPath, testVideoBytes(4096), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MODEL_SERVER_UPLOAD_URL", srv.URL)

	_, status, body, err := uploadLocalVideoToModel(1, 1, "match.mp4", videoPath, modelUploadExtras{})
	if err == nil {
		t.Fatalf("expected error for 503 response")
	}
	if status != http.StatusServiceUnavailable || len(body) == 0 {
		t.Fatalf("want 503 with body, got status=%d body=%q", status, body)
	}
}
