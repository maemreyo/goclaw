package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// defaultSTTTimeoutSeconds is the fallback timeout for STT proxy requests.
	defaultSTTTimeoutSeconds = 30

	// sttTranscribeEndpoint is appended to Voice.STTProxyURL when the URL does
	// not already end with the path.
	sttTranscribeEndpoint = "/transcribe_audio"

	// sttMaxConcurrent caps the number of simultaneous STT HTTP calls per
	// Channel instance.  When this many calls are in flight, additional callers
	// block inside acquire() until a slot is freed by release().
	//
	// A buffered channel used as a counting semaphore is the idiomatic Go pattern;
	// see https://go.dev/doc/effective_go#channels (Channels as Semaphores).
	sttMaxConcurrent = 4
)

// sttResponse is the JSON payload returned by the STT proxy on success.
type sttResponse struct {
	Transcript string `json:"transcript"`
}

// ── Shared HTTP client ────────────────────────────────────────────────────────
//
// A package-level client is shared across all Channel instances.
// Sharing one client lets the underlying Transport pool TCP connections to the
// same STT proxy host, avoiding a new dial on every audio request.
// sync.Once guarantees the client is initialised exactly once.

var (
	sttHTTPClientOnce sync.Once
	sttHTTPClient     *http.Client
)

func getSTTHTTPClient() *http.Client {
	sttHTTPClientOnce.Do(func() {
		sttHTTPClient = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10, // STT traffic targets a single host
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return sttHTTPClient
}

// ── Per-channel semaphore ─────────────────────────────────────────────────────
//
// sttSem is a counting semaphore built from a buffered channel — the idiomatic
// Go approach (Effective Go, §Channels as Semaphores):
//
//   acquire() writes into the channel; blocks when the buffer is full, i.e. when
//             sttMaxConcurrent goroutines are already inside the critical section.
//   release() reads from the channel, freeing one slot for the next waiter.
//
// sync.Once creates the channel exactly once per Channel instance.
// The zero value of sttSem is safe — call init() before first use.

type sttSem struct {
	once sync.Once
	ch   chan struct{}
}

func (s *sttSem) init() {
	s.once.Do(func() { s.ch = make(chan struct{}, sttMaxConcurrent) })
}

func (s *sttSem) acquire() { s.ch <- struct{}{} }
func (s *sttSem) release() { <-s.ch }

// ── transcribeAudio ───────────────────────────────────────────────────────────

// transcribeAudio calls the configured STT proxy with the audio file at filePath
// and returns the transcribed text.
//
// Returns ("", nil) without a network call when:
//   - Voice.STTProxyURL is empty (STT not configured), or
//   - filePath is empty (audio download failed earlier in the pipeline).
//
// Concurrency is bounded to sttMaxConcurrent simultaneous calls per Channel via a
// buffered-channel semaphore; the shared package-level http.Client pools TCP
// connections across all calls to the same STT host.
func (c *Channel) transcribeAudio(ctx context.Context, filePath string) (string, error) {
	if c.config.Voice.STTProxyURL == "" {
		return "", nil
	}
	if filePath == "" {
		return "", nil
	}

	// Acquire a concurrency slot; defer ensures release on every exit path.
	c.sttSem.init()
	c.sttSem.acquire()
	defer c.sttSem.release()

	timeoutSec := c.config.Voice.STTTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = defaultSTTTimeoutSeconds
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("stt: open audio file %q: %w", filePath, err)
	}
	defer f.Close()

	// Build multipart/form-data body.
	// Fields required by the /transcribe_audio contract:
	//   audio     — raw audio bytes
	//   tenant_id — forwarded to the proxy for auth/audit parity
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("audio", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("stt: create multipart audio field: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("stt: write audio bytes: %w", err)
	}

	tenantID := strings.TrimSpace(c.config.Voice.STTTenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	if err := w.WriteField("tenant_id", tenantID); err != nil {
		return "", fmt.Errorf("stt: write tenant_id field: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("stt: close multipart writer: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(strings.TrimSpace(c.config.Voice.STTProxyURL), "/")
	url := baseURL
	if !strings.HasSuffix(baseURL, sttTranscribeEndpoint) {
		url = baseURL + sttTranscribeEndpoint
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, &body)
	if err != nil {
		return "", fmt.Errorf("stt: build request to %q: %w", url, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.config.Voice.STTAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Voice.STTAPIKey)
	}

	slog.Debug("telegram: calling STT proxy", "url", url, "file", filepath.Base(filePath))

	resp, err := getSTTHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("stt: request to %q failed: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB safety cap
	if err != nil {
		return "", fmt.Errorf("stt: read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stt: upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result sttResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("stt: parse response JSON: %w", err)
	}

	if result.Transcript == "" {
		slog.Warn("telegram: STT transcript is empty", "url", url)
		return "", nil
	}
	slog.Info("telegram: STT transcript received", "length", len(result.Transcript))
	return result.Transcript, nil
}
