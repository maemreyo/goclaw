package cmd

import (
	"strings"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
)

// ---------------------------------------------------------------------------
// sanitizeVoiceAgentReply
// ---------------------------------------------------------------------------

// newTgCfg is a helper that builds a minimal TelegramConfig for testing.
// voiceAgentID is the value of VoiceAgentID on the channel config.
// Optionally pass custom fallback strings (empty = use built-in defaults).
func newTgCfg(voiceAgentID, fallbackTranscript, fallbackNoTranscript string) config.TelegramConfig {
	return config.TelegramConfig{
		VoiceAgentID:                   voiceAgentID,
		AudioGuardFallbackTranscript:   fallbackTranscript,
		AudioGuardFallbackNoTranscript: fallbackNoTranscript,
	}
}

const (
	testVoiceAgent = "my-voice-agent"
	dmPeer         = string(sessions.PeerDirect)
)

// TestSanitize_PassThrough_WrongAgent verifies that when the agentID does not match
// the configured VoiceAgentID, the reply is returned unchanged.
func TestSanitize_PassThrough_WrongAgent(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "<media:voice>…</media:voice>"
	reply := "system error occurred"
	got := sanitizeVoiceAgentReply(testVoiceAgent, "other-agent", "telegram", dmPeer, inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected passthrough, got %q", got)
	}
}

// TestSanitize_PassThrough_EmptyVoiceAgentID verifies that when VoiceAgentID is empty
// (feature not configured), all replies pass through untouched.
func TestSanitize_PassThrough_EmptyVoiceAgentID(t *testing.T) {
	tgCfg := newTgCfg("", "", "")
	inbound := "<media:voice>…</media:voice>"
	reply := "exit status 1"
	got := sanitizeVoiceAgentReply("", testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected passthrough when VoiceAgentID empty, got %q", got)
	}
}

// TestSanitize_PassThrough_NonTelegram verifies that non-Telegram channels are not guarded.
func TestSanitize_PassThrough_NonTelegram(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "<media:voice>…</media:voice>"
	reply := "rate limit exceeded"
	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "discord", dmPeer, inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected passthrough for non-telegram channel, got %q", got)
	}
}

// TestSanitize_PassThrough_GroupChat verifies that group chat replies are not guarded.
func TestSanitize_PassThrough_GroupChat(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "<media:voice>…</media:voice>"
	reply := "system error occurred"
	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", string(sessions.PeerGroup), inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected passthrough for group chat, got %q", got)
	}
}

// TestSanitize_PassThrough_NoAudioTag verifies that text-only inbound messages are not guarded.
func TestSanitize_PassThrough_NoAudioTag(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "just a regular text message"
	reply := "system error occurred"
	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected passthrough when no audio tag in inbound, got %q", got)
	}
}

// TestSanitize_PassThrough_CleanReply verifies that a clean (non-error) reply is not rewritten.
func TestSanitize_PassThrough_CleanReply(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "<media:voice>…</media:voice>"
	reply := "Great job! Your pronunciation is improving."
	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	if got != reply {
		t.Errorf("expected clean reply passthrough, got %q", got)
	}
}

// TestSanitize_ErrorWithTranscript_DefaultFallback verifies that when a transcript is available
// and no custom fallback is configured, the built-in English message is used.
func TestSanitize_ErrorWithTranscript_DefaultFallback(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := `<media:voice><transcript>I usually wake up at seven</transcript></media:voice>`
	reply := "system error: tool execution failed"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)

	// The default fallback must contain the transcript text.
	if !contains(got, "I usually wake up at seven") {
		t.Errorf("expected transcript in fallback, got: %q", got)
	}
	// Must not contain the original technical error.
	if contains(got, "system error") {
		t.Errorf("technical error leaked into fallback: %q", got)
	}
}

// TestSanitize_ErrorWithTranscript_CustomFallback verifies that a custom fallback template
// from TelegramConfig is used when set.
func TestSanitize_ErrorWithTranscript_CustomFallback(t *testing.T) {
	customTpl := "Transcript received: %s. Please send again!"
	tgCfg := newTgCfg(testVoiceAgent, customTpl, "")
	inbound := `<media:voice><transcript>hello world</transcript></media:voice>`
	reply := "rate limit exceeded"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	want := "Transcript received: hello world. Please send again!"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestSanitize_ErrorNoTranscript_DefaultFallback verifies the no-transcript default path.
func TestSanitize_ErrorNoTranscript_DefaultFallback(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := "<media:voice>…</media:voice>" // no transcript block
	reply := "exit status 1"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)

	// Must not contain the original technical error.
	if contains(got, "exit status") {
		t.Errorf("technical error leaked into fallback: %q", got)
	}
	// Must be non-empty.
	if got == "" {
		t.Error("expected non-empty fallback, got empty string")
	}
}

// TestSanitize_ErrorNoTranscript_CustomFallback verifies the no-transcript custom message path.
func TestSanitize_ErrorNoTranscript_CustomFallback(t *testing.T) {
	custom := "Sorry, please resend your voice note."
	tgCfg := newTgCfg(testVoiceAgent, "", custom)
	inbound := "<media:audio>…</media:audio>"
	reply := "tool error: service unavailable"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	if got != custom {
		t.Errorf("expected custom no-transcript fallback %q, got %q", custom, got)
	}
}

// TestSanitize_MediaAudioTag verifies that <media:audio> is treated the same as <media:voice>.
func TestSanitize_MediaAudioTag(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	inbound := `<media:audio><transcript>good morning</transcript></media:audio>`
	reply := "rate limit: too many requests"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	if contains(got, "rate limit") {
		t.Errorf("technical error leaked: %q", got)
	}
	if !contains(got, "good morning") {
		t.Errorf("expected transcript in fallback, got: %q", got)
	}
}

// TestSanitize_ErrorWithTranscript_CustomFallbackNoPlaceholder verifies that a
// custom fallback template WITHOUT a %s placeholder does NOT produce
// "%!(EXTRA string=...)" garbage. The transcript is silently omitted but the
// student receives a clean message.
func TestSanitize_ErrorWithTranscript_CustomFallbackNoPlaceholder(t *testing.T) {
	// Operator set a clean message with no %s — common mistake.
	customTpl := "Please resend your voice note, there was a small hiccup!"
	tgCfg := newTgCfg(testVoiceAgent, customTpl, "")
	inbound := `<media:voice><transcript>hello world</transcript></media:voice>`
	reply := "system error: tool execution failed"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)

	// Must be exactly the template string — no %!(EXTRA...) suffix appended.
	if got != customTpl {
		t.Errorf("expected clean fallback %q, got %q", customTpl, got)
	}
	// Explicit check: fmt.Sprintf leakage looks like "%!(EXTRA".
	if strings.Contains(got, "%!") {
		t.Errorf("fmt.Sprintf garbage leaked into output: %q", got)
	}
}

// TestSanitize_ErrorWithTranscript_CustomFallbackWithPlaceholder verifies that
// a custom template WITH %s correctly inlines the transcript.
func TestSanitize_ErrorWithTranscript_CustomFallbackWithPlaceholder(t *testing.T) {
	customTpl := `Mình nghe: "%s" — gửi lại nhé!`
	tgCfg := newTgCfg(testVoiceAgent, customTpl, "")
	inbound := `<media:voice><transcript>xin chào</transcript></media:voice>`
	reply := "exit status 1"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)
	want := `Mình nghe: "xin chào" — gửi lại nhé!`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestSanitize_ErrorWithTranscript_DefaultFallbackNoFmtGarbage verifies that
// the built-in default also inlines transcript cleanly after switching from
// fmt.Sprintf to strings.ReplaceAll.
func TestSanitize_ErrorWithTranscript_DefaultFallbackNoFmtGarbage(t *testing.T) {
	tgCfg := newTgCfg(testVoiceAgent, "", "")
	transcript := "I wake up at 7am every day"
	inbound := "<media:voice><transcript>" + transcript + "</transcript></media:voice>"
	reply := "tool error: evaluation failed"

	got := sanitizeVoiceAgentReply(testVoiceAgent, testVoiceAgent, "telegram", dmPeer, inbound, reply, tgCfg)

	if strings.Contains(got, "%!") {
		t.Errorf("fmt.Sprintf garbage in default fallback: %q", got)
	}
	if !strings.Contains(got, transcript) {
		t.Errorf("expected transcript %q in fallback, got: %q", transcript, got)
	}
}

// ---------------------------------------------------------------------------
// containsTechnicalErrorLanguage
// ---------------------------------------------------------------------------

func TestContainsTechnicalError_Positives(t *testing.T) {
	cases := []string{
		"vấn đề kỹ thuật xảy ra",
		"lỗi hệ thống",
		"vấn đề hệ thống",
		"technical issue detected",
		"system error: something broke",
		"exit status 1",
		"rate limit exceeded",
		"api rate limit hit",
		"tool error: execution failed",
		// mixed case
		"SYSTEM ERROR occurred",
		"Rate Limit Exceeded",
	}
	for _, s := range cases {
		if !containsTechnicalErrorLanguage(s) {
			t.Errorf("expected true for %q, got false", s)
		}
	}
}

func TestContainsTechnicalError_Negatives(t *testing.T) {
	cases := []string{
		"",
		"Great job!",
		"Your pronunciation is improving.",
		"Please try again.",
		"I heard you say: hello world.",
	}
	for _, s := range cases {
		if containsTechnicalErrorLanguage(s) {
			t.Errorf("expected false for %q, got true", s)
		}
	}
}

// ---------------------------------------------------------------------------
// extractTranscriptFromInbound
// ---------------------------------------------------------------------------

func TestExtractTranscript_Present(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `<media:voice><transcript>hello world</transcript></media:voice>`,
			want:  "hello world",
		},
		{
			input: `<media:audio><transcript>  spaces around  </transcript></media:audio>`,
			want:  "spaces around",
		},
		{
			input: "<media:voice>\n<transcript>\nMulti\nline\ntranscript\n</transcript>\n</media:voice>",
			want:  "Multi line transcript",
		},
		{
			input: "<transcript>only transcript</transcript>",
			want:  "only transcript",
		},
	}
	for _, tc := range cases {
		got := extractTranscriptFromInbound(tc.input)
		if got != tc.want {
			t.Errorf("input %q: expected %q, got %q", tc.input, tc.want, got)
		}
	}
}

func TestExtractTranscript_Absent(t *testing.T) {
	cases := []string{
		"<media:voice>…</media:voice>",
		"plain text message",
		"",
	}
	for _, s := range cases {
		got := extractTranscriptFromInbound(s)
		if got != "" {
			t.Errorf("expected empty transcript for %q, got %q", s, got)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
