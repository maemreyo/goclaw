package voiceguard_test

import (
	"strings"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/channels/telegram/voiceguard"
	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

const testAgent = "my-voice-agent"

func voiceCfg(
	fallbackTranscript,
	fallbackNoTranscript string,
	markers []string,
) config.TelegramVoiceConfig {
	return config.TelegramVoiceConfig{
		AgentID:                        testAgent,
		AudioGuardFallbackTranscript:   fallbackTranscript,
		AudioGuardFallbackNoTranscript: fallbackNoTranscript,
		AudioGuardErrorMarkers:         markers,
	}
}

func sanitize(inbound, reply string, cfg config.TelegramVoiceConfig) string {
	return voiceguard.SanitizeReply(testAgent, testAgent, "telegram", "direct", inbound, reply, cfg)
}

// ── Pass-through: guard must not fire ────────────────────────────────────────

func TestSanitize_PassThrough_WrongAgent(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "system error"
	got := voiceguard.SanitizeReply(testAgent, "other-agent", "telegram", "direct", inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("wrong agent: expected passthrough, got %q", got)
	}
}

func TestSanitize_PassThrough_EmptyVoiceAgentID(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "exit status 1"
	got := voiceguard.SanitizeReply("", testAgent, "telegram", "direct", inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("empty voiceAgentID: expected passthrough, got %q", got)
	}
}

func TestSanitize_PassThrough_NonTelegram(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "rate limit exceeded"
	got := voiceguard.SanitizeReply(testAgent, testAgent, "discord", "direct", inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("non-telegram channel: expected passthrough, got %q", got)
	}
}

func TestSanitize_PassThrough_GroupChat(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "system error occurred"
	got := voiceguard.SanitizeReply(testAgent, testAgent, "telegram", "group", inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("group chat: expected passthrough, got %q", got)
	}
}

func TestSanitize_PassThrough_NoAudioTag(t *testing.T) {
	inbound := "just a regular text message"
	reply := "technical issue in processing"
	got := sanitize(inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("text-only inbound: expected passthrough, got %q", got)
	}
}

func TestSanitize_PassThrough_CleanReply(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "Great pronunciation! Keep going."
	got := sanitize(inbound, reply, voiceCfg("", "", nil))
	if got != reply {
		t.Errorf("clean reply: expected passthrough, got %q", got)
	}
}

// ── Guard fires: default fallbacks ───────────────────────────────────────────

func TestSanitize_DefaultFallback_WithTranscript(t *testing.T) {
	inbound := `<media:voice>…</media:voice><transcript>hello world</transcript>`
	reply := "system error occurred"
	got := sanitize(inbound, reply, voiceCfg("", "", nil))
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected transcript in fallback, got %q", got)
	}
}

func TestSanitize_DefaultFallback_NoTranscript(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "exit status 1 — tool error"
	got := sanitize(inbound, reply, voiceCfg("", "", nil))
	if got == reply {
		t.Error("expected fallback, got original reply unchanged")
	}
	if got == "" {
		t.Error("fallback must not be empty")
	}
}

// ── Guard fires: custom fallbacks ────────────────────────────────────────────

func TestSanitize_CustomFallback_WithPlaceholder(t *testing.T) {
	inbound := `<media:voice>…</media:voice><transcript>xin chào</transcript>`
	reply := "lỗi hệ thống nghiêm trọng"
	customTpl := `Tôi nghe được: "%s". Vui lòng thử lại!`
	got := sanitize(inbound, reply, voiceCfg(customTpl, "", nil))
	want := `Tôi nghe được: "xin chào". Vui lòng thử lại!`
	if got != want {
		t.Errorf("custom fallback:\n  got  %q\n  want %q", got, want)
	}
}

func TestSanitize_CustomFallback_NoPlaceholder(t *testing.T) {
	// Template without %s — strings.ReplaceAll must not produce garbage.
	inbound := `<media:voice>…</media:voice><transcript>xin chào</transcript>`
	reply := "system error"
	customTpl := "Vui lòng gửi lại nhé!"
	got := sanitize(inbound, reply, voiceCfg(customTpl, "", nil))
	if got != customTpl {
		t.Errorf("no-placeholder template: expected %q verbatim, got %q", customTpl, got)
	}
}

// ── Custom error markers: REPLACES behaviour ──────────────────────────────────

func TestSanitize_CustomMarkers_Trigger(t *testing.T) {
	inbound := "<media:voice>…</media:voice>"
	reply := "deployment pipeline aborted"
	got := sanitize(inbound, reply, voiceCfg("", "", []string{"deployment pipeline"}))
	if got == reply {
		t.Error("custom marker: expected fallback, got original reply")
	}
}

func TestSanitize_CustomMarkers_ReplacesDefaults(t *testing.T) {
	// When custom markers are set, defaultErrorMarkers must NOT fire.
	// "system error" is in the default list but not in the custom list below.
	inbound := "<media:voice>…</media:voice>"
	reply := "system error"
	got := sanitize(inbound, reply, voiceCfg("", "", []string{"only-this-marker"}))
	if got != reply {
		t.Errorf("custom markers should replace defaults: expected passthrough for %q, got %q", reply, got)
	}
}

// ── Audio tag variants ────────────────────────────────────────────────────────

func TestSanitize_AudioTag_AlsoTriggers(t *testing.T) {
	inbound := "<media:audio>…</media:audio>"
	reply := "system error"
	got := sanitize(inbound, reply, voiceCfg("", "", nil))
	if got == reply {
		t.Error("media:audio tag: expected fallback, got original reply")
	}
}
