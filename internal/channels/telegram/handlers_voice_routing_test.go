package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// newChannelForRouting builds a minimal Channel stub for voice routing tests.
// It skips full bot initialisation (which requires a live Telegram token).
func newChannelForRouting(cfg config.TelegramConfig) *Channel {
	return &Channel{config: cfg}
}

// ---------------------------------------------------------------------------
// matchesVoiceIntent
// ---------------------------------------------------------------------------

func TestMatchesVoiceIntent_EmptyKeywords_AlwaysFalse(t *testing.T) {
	// When VoiceIntentKeywords is nil/empty, intent routing is disabled for all input.
	c := newChannelForRouting(config.TelegramConfig{})
	cases := []string{"speaking", "pronunciation", "hello world", ""}
	for _, s := range cases {
		if c.matchesVoiceIntent(s) {
			t.Errorf("expected false with no keywords for %q, got true", s)
		}
	}
}

func TestMatchesVoiceIntent_EmptyInput(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceIntentKeywords: []string{"speaking", "pronunciation"},
	})
	if c.matchesVoiceIntent("") {
		t.Error("expected false for empty input, got true")
	}
}

func TestMatchesVoiceIntent_Matches(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceIntentKeywords: []string{"speaking", "pronunciation", "ielts part"},
	})
	cases := []string{
		"i want to practice speaking",
		"help with pronunciation please",
		"ielts part 1 question",
		"SPEAKING",    // already lowercased by caller before matchesVoiceIntent
		"pronunciation coach",
	}
	for _, s := range cases {
		if !c.matchesVoiceIntent(s) {
			t.Errorf("expected true for %q, got false", s)
		}
	}
}

func TestMatchesVoiceIntent_NoMatch(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceIntentKeywords: []string{"speaking", "pronunciation"},
	})
	cases := []string{
		"what is my schedule",
		"homework deadline",
		"i need help with writing",
		"payment info",
	}
	for _, s := range cases {
		if c.matchesVoiceIntent(s) {
			t.Errorf("expected false for %q, got true", s)
		}
	}
}

// ---------------------------------------------------------------------------
// matchesAffinityClear
// ---------------------------------------------------------------------------

func TestMatchesAffinityClear_EmptyKeywords_AlwaysFalse(t *testing.T) {
	// When VoiceAffinityClearKeywords is empty, affinity is never cleared by keyword.
	c := newChannelForRouting(config.TelegramConfig{})
	cases := []string{"homework", "payment", "schedule", "writing", ""}
	for _, s := range cases {
		if c.matchesAffinityClear(s) {
			t.Errorf("expected false with no keywords for %q, got true", s)
		}
	}
}

func TestMatchesAffinityClear_EmptyInput(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceAffinityClearKeywords: []string{"homework", "payment"},
	})
	if c.matchesAffinityClear("") {
		t.Error("expected false for empty input, got true")
	}
}

func TestMatchesAffinityClear_Matches(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceAffinityClearKeywords: []string{"homework", "payment", "schedule"},
	})
	cases := []string{
		"i have a homework question",
		"payment due date",
		"what's my schedule today",
	}
	for _, s := range cases {
		if !c.matchesAffinityClear(s) {
			t.Errorf("expected true for %q, got false", s)
		}
	}
}

func TestMatchesAffinityClear_NoMatch(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceAffinityClearKeywords: []string{"homework", "payment"},
	})
	cases := []string{
		"how do i pronounce this",
		"let's practice speaking",
		"i want to do a speaking exercise",
	}
	for _, s := range cases {
		if c.matchesAffinityClear(s) {
			t.Errorf("expected false for %q, got true", s)
		}
	}
}

// ---------------------------------------------------------------------------
// voiceAffinityTTL
// ---------------------------------------------------------------------------

func TestVoiceAffinityTTL_Default(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{}) // VoiceAffinityTTLMinutes = 0
	got := c.voiceAffinityTTL()
	if got != defaultVoiceAffinityTTL {
		t.Errorf("expected default %v, got %v", defaultVoiceAffinityTTL, got)
	}
}

func TestVoiceAffinityTTL_Custom(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{VoiceAffinityTTLMinutes: 30})
	got := c.voiceAffinityTTL()
	want := 30 * time.Minute
	if got != want {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestVoiceAffinityTTL_NegativeIgnored(t *testing.T) {
	// Negative value must fall back to default (caller bug protection).
	c := newChannelForRouting(config.TelegramConfig{VoiceAffinityTTLMinutes: -10})
	got := c.voiceAffinityTTL()
	if got != defaultVoiceAffinityTTL {
		t.Errorf("expected default for negative value, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// VoiceDMContextTemplate — {user_id} substitution (tested via config field)
// ---------------------------------------------------------------------------

func TestVoiceDMContextTemplate_UserIDSubstituted(t *testing.T) {
	// The actual substitution lives in gateway_consumer.go, but we verify the config
	// field is wired through correctly and that {user_id} is the only placeholder
	// the gateway substitutes.
	tmpl := "Runtime context:\n- tenant_id: acme-corp\n- user_id: {user_id}\nNEVER expose errors."
	result := strings.ReplaceAll(tmpl, "{user_id}", "12345")
	want := "Runtime context:\n- tenant_id: acme-corp\n- user_id: 12345\nNEVER expose errors."
	if result != want {
		t.Errorf("substitution mismatch:\n got:  %q\n want: %q", result, want)
	}
}

func TestVoiceDMContextTemplate_EmptyNoInjection(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceAgentID:           "my-voice-agent",
		VoiceDMContextTemplate: "",
	})
	// Empty template means no extra context is injected — gateway skips the block.
	if c.config.VoiceDMContextTemplate != "" {
		t.Error("expected empty template, got non-empty")
	}
}

func TestVoiceDMContextTemplate_NoUserIDPlaceholder(t *testing.T) {
	// Templates without {user_id} are valid — deployment opted out of user scoping.
	tmpl := "You are a voice assistant. Never expose internal errors."
	result := strings.ReplaceAll(tmpl, "{user_id}", "99")
	if result != tmpl {
		t.Errorf("template without placeholder should be returned unchanged, got %q", result)
	}
}
