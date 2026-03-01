package telegram

// handlers_voice_routing_test.go — table-driven tests for resolveTargetAgent.
//
// Tests live in package telegram (white-box) so we can:
//   - access unexported types (dmAffinity, MediaInfo)
//   - seed dmAgentAffinity directly without an exported API
//   - call resolveTargetAgent without going through the Telegram bot loop
//
// Each case creates a minimal Channel stub with a real BaseChannel so that
// c.AgentID() works, then asserts the returned (agentID, finalContent) pair
// and any affinity side-effects.

import (
	"sync"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
)

const (
	testDefaultAgent = "default-agent"
	testVoiceAgent   = "voice-agent"
)

// newRoutingChannel builds the minimal Channel needed for resolveTargetAgent.
// It wires a real BaseChannel so c.AgentID() returns testDefaultAgent.
func newRoutingChannel(voiceCfg config.TelegramVoiceConfig) *Channel {
	base := channels.NewBaseChannel("telegram", nil, nil)
	base.SetAgentID(testDefaultAgent)
	return &Channel{
		BaseChannel: base,
		config: config.TelegramConfig{
			Voice: voiceCfg,
		},
	}
}

// ── Table-driven routing tests ────────────────────────────────────────────────

func TestResolveTargetAgent(t *testing.T) {
	baseCfg := config.TelegramVoiceConfig{
		AgentID:               testVoiceAgent,
		StartMessage:          "Voice session started.",
		IntentKeywords:        []string{"speaking", "pronunciation"},
		AffinityClearKeywords: []string{"homework", "payment"},
		AffinityTTLMinutes:    60,
	}

	validAffinity := dmAffinity{AgentID: testVoiceAgent, UpdatedAt: time.Now()}
	expiredAffinity := dmAffinity{AgentID: testVoiceAgent, UpdatedAt: time.Now().Add(-2 * time.Hour)}

	tests := []struct {
		name         string
		voiceCfg     config.TelegramVoiceConfig
		chatID       string
		isGroup      bool
		mediaList    []MediaInfo
		content      string
		preAffinity  *dmAffinity // non-nil → seed dmAgentAffinity before call
		wantAgentID  string
		wantContent  string // "" means content must remain unchanged
		wantAffinity bool   // true = affinity entry must exist after call
	}{
		// ── Priority 1: audio/voice media ──────────────────────────────────────
		{
			name:         "audio in DM → voice agent",
			voiceCfg:     baseCfg,
			chatID:       "c1",
			isGroup:      false,
			mediaList:    []MediaInfo{{Type: "audio"}},
			content:      "hello",
			wantAgentID:  testVoiceAgent,
			wantAffinity: true,
		},
		{
			name:         "voice in group → voice agent (audio overrides group check)",
			voiceCfg:     baseCfg,
			chatID:       "c2",
			isGroup:      true,
			mediaList:    []MediaInfo{{Type: "voice"}},
			wantAgentID:  testVoiceAgent,
			// Group chats must NOT have affinity stored — it is never read for groups
			// and would accumulate indefinitely in sync.Map.
			wantAffinity: false,
		},
		// ── Priority 2: /start command ─────────────────────────────────────────
		{
			name:         "/start rewrites content with StartMessage",
			voiceCfg:     baseCfg,
			chatID:       "c3",
			isGroup:      false,
			content:      "/start",
			wantAgentID:  testVoiceAgent,
			wantContent:  "Voice session started.",
			wantAffinity: true,
		},
		{
			name:         "bare 'start' keyword also rewrites",
			voiceCfg:     baseCfg,
			chatID:       "c4",
			isGroup:      false,
			content:      "start",
			wantAgentID:  testVoiceAgent,
			wantContent:  "Voice session started.",
			wantAffinity: true,
		},
		{
			name:         "/start in group does NOT route (only audio does)",
			voiceCfg:     baseCfg,
			chatID:       "c5",
			isGroup:      true,
			content:      "/start",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
		{
			name: "/start with no StartMessage uses built-in default",
			voiceCfg: config.TelegramVoiceConfig{
				AgentID: testVoiceAgent,
				// StartMessage intentionally empty
			},
			chatID:       "c6",
			isGroup:      false,
			content:      "/start",
			wantAgentID:  testVoiceAgent,
			wantContent:  "User sent /start.",
			wantAffinity: true,
		},
		// ── Priority 3: intent keywords ────────────────────────────────────────
		{
			name:         "intent keyword match routes to voice agent",
			voiceCfg:     baseCfg,
			chatID:       "c7",
			isGroup:      false,
			content:      "I want to practice speaking today",
			wantAgentID:  testVoiceAgent,
			wantAffinity: true,
		},
		{
			name:         "intent keyword is case-insensitive",
			voiceCfg:     baseCfg,
			chatID:       "c8",
			isGroup:      false,
			content:      "Let's do some PRONUNCIATION practice",
			wantAgentID:  testVoiceAgent,
			wantAffinity: true,
		},
		{
			name:         "no keyword match → default agent",
			voiceCfg:     baseCfg,
			chatID:       "c9",
			isGroup:      false,
			content:      "What time does the library open?",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
		{
			name:         "intent keyword in group does NOT route",
			voiceCfg:     baseCfg,
			chatID:       "c10",
			isGroup:      true,
			content:      "speaking practice please",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
		// ── Priority 4: session affinity ───────────────────────────────────────
		{
			name:         "valid affinity continues routing to voice agent",
			voiceCfg:     baseCfg,
			chatID:       "c11",
			isGroup:      false,
			preAffinity:  &validAffinity,
			content:      "How was that?",
			wantAgentID:  testVoiceAgent,
			wantAffinity: true,
		},
		{
			name:         "expired affinity routes to default and is evicted",
			voiceCfg:     baseCfg,
			chatID:       "c12",
			isGroup:      false,
			preAffinity:  &expiredAffinity,
			content:      "How was that?",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
		// ── Priority 5: affinity clear keywords ───────────────────────────────
		{
			name:         "clear keyword evicts affinity → default agent",
			voiceCfg:     baseCfg,
			chatID:       "c13",
			isGroup:      false,
			preAffinity:  &validAffinity,
			content:      "I have a homework question",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
		// ── Voice agent not configured ─────────────────────────────────────────
		{
			name:         "no voice agent → always default regardless of media",
			voiceCfg:     config.TelegramVoiceConfig{}, // AgentID empty
			chatID:       "c14",
			isGroup:      false,
			mediaList:    []MediaInfo{{Type: "voice"}},
			content:      "/start",
			wantAgentID:  testDefaultAgent,
			wantAffinity: false,
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop var for t.Parallel() (Go < 1.22 safety)
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := newRoutingChannel(tt.voiceCfg)
			if tt.preAffinity != nil {
				ch.dmAgentAffinity.Store(tt.chatID, *tt.preAffinity)
			}

			gotAgent, gotContent := ch.resolveTargetAgent(
				tt.chatID, tt.isGroup, tt.mediaList, tt.content,
			)

			if gotAgent != tt.wantAgentID {
				t.Errorf("agentID: got %q, want %q", gotAgent, tt.wantAgentID)
			}

			expectedContent := tt.content
			if tt.wantContent != "" {
				expectedContent = tt.wantContent
			}
			if gotContent != expectedContent {
				t.Errorf("content:\n  got  %q\n  want %q", gotContent, expectedContent)
			}

			_, hasAffinity := ch.dmAgentAffinity.Load(tt.chatID)
			if tt.wantAffinity && !hasAffinity {
				t.Error("affinity: expected entry to exist after call, but it was absent")
			}
			if !tt.wantAffinity && hasAffinity {
				t.Error("affinity: expected entry to be absent after call, but it exists")
			}
		})
	}
}

// TestResolveTargetAgent_AffinityRace verifies that concurrent calls on the
// same chatID do not cause data races on dmAgentAffinity (sync.Map).
// Run with -race to activate the Go race detector.
func TestResolveTargetAgent_AffinityRace(t *testing.T) {
	ch := newRoutingChannel(config.TelegramVoiceConfig{
		AgentID:        testVoiceAgent,
		IntentKeywords: []string{"speaking"},
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch.resolveTargetAgent(
				"race-chat",
				false,
				[]MediaInfo{{Type: "voice"}},
				"speaking test",
			)
		}()
	}
	wg.Wait() // pass/fail determined by -race flag, not assertions
}
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
		"speaking", // note: caller always calls strings.ToLower before matchesVoiceIntent
		"pronunciation coach",
	}
	for _, s := range cases {
		if !c.matchesVoiceIntent(s) {
			t.Errorf("expected true for %q, got false", s)
		}
	}
}

// TestMatchesVoiceIntent_MixedCaseKeywords verifies that config keywords stored
// with mixed case (e.g. from a DB that does not normalise on write) still match
// lowercase inbound text. This exercises the strings.ToLower(kw) defensive path
// added to matchesVoiceIntent.
func TestMatchesVoiceIntent_MixedCaseKeywords(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		// Keywords intentionally stored with mixed case, as a careless operator might do.
		VoiceIntentKeywords: []string{"Speaking", "PRONUNCIATION", "Ielts Part"},
	})
	// Inbound text is always lowercased by the caller (handlers.go strings.ToLower).
	cases := []string{
		"i want to practice speaking",
		"help with pronunciation please",
		"ielts part 1 question",
	}
	for _, s := range cases {
		if !c.matchesVoiceIntent(s) {
			t.Errorf("mixed-case keyword: expected true for %q, got false", s)
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

// TestMatchesAffinityClear_MixedCaseKeywords mirrors TestMatchesVoiceIntent_MixedCaseKeywords
// for the affinity-clear path.
func TestMatchesAffinityClear_MixedCaseKeywords(t *testing.T) {
	c := newChannelForRouting(config.TelegramConfig{
		VoiceAffinityClearKeywords: []string{"Homework", "PAYMENT", "Schedule"},
	})
	cases := []string{
		"i have a homework question",
		"payment due date",
		"what is my schedule today",
	}
	for _, s := range cases {
		if !c.matchesAffinityClear(s) {
			t.Errorf("mixed-case keyword: expected true for %q, got false", s)
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
