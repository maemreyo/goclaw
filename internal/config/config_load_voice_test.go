package config_test

// config_load_voice_test.go — verifies that all 5 voice-agent env vars are
// wired through applyEnvOverrides into Config.Channels.Telegram.
//
// These tests protect against the managed-mode regression where
// GOCLAW_VOICE_AGENT_ID was missing, causing VoiceDMContextTemplate injection
// and AudioGuard sanitization to be silently skipped at runtime.
// See gateway_consumer.go lines 156 and 249 for the gates that depend on this.

import (
	"os"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// setEnv sets KEY=VALUE for the duration of the test and restores on cleanup.
func setEnv(t *testing.T, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatal("setEnv: odd number of arguments")
	}
	for i := 0; i < len(pairs); i += 2 {
		key, val := pairs[i], pairs[i+1]
		prev, existed := os.LookupEnv(key)
		if err := os.Setenv(key, val); err != nil {
			t.Fatalf("setEnv Setenv(%s): %v", key, err)
		}
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, prev)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}

// TestVoiceAgentIDEnvOverride is the critical regression test: GOCLAW_VOICE_AGENT_ID
// must populate cfg.Channels.Telegram.VoiceAgentID so that gateway_consumer.go's
// injection/sanitize gates fire correctly in managed mode.
//
// Before the fix, this env var did not exist in applyEnvOverrides, so
// VoiceAgentID was always "" and both voice features were dead code.
func TestVoiceAgentIDEnvOverride(t *testing.T) {
	setEnv(t, "GOCLAW_VOICE_AGENT_ID", "speaking-agent")
	cfg := config.Default()
	cfg.ApplyEnvOverrides()
	if got := cfg.Channels.Telegram.VoiceAgentID; got != "speaking-agent" {
		t.Errorf("GOCLAW_VOICE_AGENT_ID: expected %q, got %q", "speaking-agent", got)
	}
}

// TestSTTTenantIDEnvOverride verifies the existing GOCLAW_STT_TENANT_ID override.
func TestSTTTenantIDEnvOverride(t *testing.T) {
	setEnv(t, "GOCLAW_STT_TENANT_ID", "my-school")
	cfg := config.Default()
	cfg.ApplyEnvOverrides()
	if got := cfg.Channels.Telegram.STTTenantID; got != "my-school" {
		t.Errorf("GOCLAW_STT_TENANT_ID: expected %q, got %q", "my-school", got)
	}
}

// TestVoiceDMContextTemplateEnvOverride verifies the existing template override.
func TestVoiceDMContextTemplateEnvOverride(t *testing.T) {
	tmpl := "Runtime context:\n- user_id: {user_id}"
	setEnv(t, "GOCLAW_VOICE_DM_CONTEXT_TEMPLATE", tmpl)
	cfg := config.Default()
	cfg.ApplyEnvOverrides()
	if got := cfg.Channels.Telegram.VoiceDMContextTemplate; got != tmpl {
		t.Errorf("GOCLAW_VOICE_DM_CONTEXT_TEMPLATE: expected %q, got %q", tmpl, got)
	}
}

// TestAudioGuardFallbackEnvOverrides verifies the two audio-guard fallback overrides.
// In managed mode these provide Vietnamese deployment-specific messages without
// requiring a config.json file.
func TestAudioGuardFallbackEnvOverrides(t *testing.T) {
	setEnv(t,
		"GOCLAW_AUDIO_GUARD_FALLBACK_TRANSCRIPT", "Got it: %s — please resend",
		"GOCLAW_AUDIO_GUARD_FALLBACK_NO_TRANSCRIPT", "Got your voice — please resend",
	)
	cfg := config.Default()
	cfg.ApplyEnvOverrides()
	if got := cfg.Channels.Telegram.AudioGuardFallbackTranscript; got != "Got it: %s — please resend" {
		t.Errorf("GOCLAW_AUDIO_GUARD_FALLBACK_TRANSCRIPT: got %q", got)
	}
	if got := cfg.Channels.Telegram.AudioGuardFallbackNoTranscript; got != "Got your voice — please resend" {
		t.Errorf("GOCLAW_AUDIO_GUARD_FALLBACK_NO_TRANSCRIPT: got %q", got)
	}
}

// TestVoiceEnvOverridesDoNotClobberConfigFileValues verifies that an empty env
// var does NOT overwrite a value already set (e.g. from config.json).
// envStr only writes when the env var is non-empty.
func TestVoiceEnvOverridesDoNotClobberConfigFileValues(t *testing.T) {
	cfg := config.Default()
	cfg.Channels.Telegram.VoiceAgentID = "my-custom-agent"
	os.Unsetenv("GOCLAW_VOICE_AGENT_ID")
	cfg.ApplyEnvOverrides()
	if got := cfg.Channels.Telegram.VoiceAgentID; got != "my-custom-agent" {
		t.Errorf("empty env var should not overwrite config: expected %q, got %q", "my-custom-agent", got)
	}
}

// TestAllVoiceEnvVarsTogether verifies all 5 voice env vars applied simultaneously,
// matching the full set a managed-mode deployment (like EduOS) would set.
func TestAllVoiceEnvVarsTogether(t *testing.T) {
	setEnv(t,
		"GOCLAW_VOICE_AGENT_ID", "speaking-agent",
		"GOCLAW_STT_TENANT_ID", "edu-tenant",
		"GOCLAW_VOICE_DM_CONTEXT_TEMPLATE", "ctx: {user_id}",
		"GOCLAW_AUDIO_GUARD_FALLBACK_TRANSCRIPT", "heard: %s",
		"GOCLAW_AUDIO_GUARD_FALLBACK_NO_TRANSCRIPT", "resend please",
	)
	cfg := config.Default()
	cfg.ApplyEnvOverrides()

	tg := cfg.Channels.Telegram
	if tg.VoiceAgentID != "speaking-agent" {
		t.Errorf("VoiceAgentID: got %q", tg.VoiceAgentID)
	}
	if tg.STTTenantID != "edu-tenant" {
		t.Errorf("STTTenantID: got %q", tg.STTTenantID)
	}
	if tg.VoiceDMContextTemplate != "ctx: {user_id}" {
		t.Errorf("VoiceDMContextTemplate: got %q", tg.VoiceDMContextTemplate)
	}
	if tg.AudioGuardFallbackTranscript != "heard: %s" {
		t.Errorf("AudioGuardFallbackTranscript: got %q", tg.AudioGuardFallbackTranscript)
	}
	if tg.AudioGuardFallbackNoTranscript != "resend please" {
		t.Errorf("AudioGuardFallbackNoTranscript: got %q", tg.AudioGuardFallbackNoTranscript)
	}
}
