package telegram

import (
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// telegramCreds maps the credentials JSON from the channel_instances table.
type telegramCreds struct {
	Token string `json:"token"`
	Proxy string `json:"proxy,omitempty"`
}

// telegramInstanceConfig maps the non-secret config JSONB from the channel_instances table.
// It supports two JSON layouts for voice settings:
//   - Nested (preferred for new rows):  {"voice": {"agent_id": "speaking-agent", ...}}
//   - Flat  (legacy, still accepted):   {"voice_agent_id": "speaking-agent", ...}
//
// buildChannel promotes flat fields into the nested Voice struct when Voice.AgentID is empty,
// so existing DB rows continue to work without migration.
type telegramInstanceConfig struct {
	DMPolicy       string   `json:"dm_policy,omitempty"`
	GroupPolicy    string   `json:"group_policy,omitempty"`
	RequireMention *bool    `json:"require_mention,omitempty"`
	HistoryLimit   int      `json:"history_limit,omitempty"`
	StreamMode     string   `json:"stream_mode,omitempty"`
	ReactionLevel  string   `json:"reaction_level,omitempty"`
	MediaMaxBytes  int64    `json:"media_max_bytes,omitempty"`
	LinkPreview    *bool    `json:"link_preview,omitempty"`
	AllowFrom      []string `json:"allow_from,omitempty"`

	// Nested voice config — preferred layout for new DB rows.
	Voice config.TelegramVoiceConfig `json:"voice,omitempty"`

	// Legacy flat fields — populated by older DB rows.
	// buildChannel promotes these into Voice when Voice.AgentID is empty.
	LegacySTTProxyURL                    string   `json:"stt_proxy_url,omitempty"`
	LegacySTTAPIKey                      string   `json:"stt_api_key,omitempty"`
	LegacySTTTenantID                    string   `json:"stt_tenant_id,omitempty"`
	LegacySTTTimeoutSec                  int      `json:"stt_timeout_seconds,omitempty"`
	LegacyVoiceAgentID                   string   `json:"voice_agent_id,omitempty"`
	LegacyVoiceStartMessage              string   `json:"voice_start_message,omitempty"`
	LegacyVoiceIntentKeywords            []string `json:"voice_intent_keywords,omitempty"`
	LegacyVoiceAffinityClearKeywords     []string `json:"voice_affinity_clear_keywords,omitempty"`
	LegacyVoiceAffinityTTLMinutes        int      `json:"voice_affinity_ttl_minutes,omitempty"`
	LegacyVoiceDMContextTemplate         string   `json:"voice_dm_context_template,omitempty"`
	LegacyAudioGuardFallbackTranscript   string   `json:"audio_guard_fallback_transcript,omitempty"`
	LegacyAudioGuardFallbackNoTranscript string   `json:"audio_guard_fallback_no_transcript,omitempty"`
}

// Factory creates a Telegram channel from DB instance data (no agent/team store).
func Factory(name string, creds json.RawMessage, cfg json.RawMessage,
	msgBus *bus.MessageBus, pairingSvc store.PairingStore) (channels.Channel, error) {
	return buildChannel(name, creds, cfg, msgBus, pairingSvc, nil, nil)
}

// FactoryWithStores returns a ChannelFactory that includes agent and team stores
// for group file writer management and /tasks, /task_detail commands.
func FactoryWithStores(agentStore store.AgentStore, teamStore store.TeamStore) channels.ChannelFactory {
	return func(name string, creds json.RawMessage, cfg json.RawMessage,
		msgBus *bus.MessageBus, pairingSvc store.PairingStore) (channels.Channel, error) {
		return buildChannel(name, creds, cfg, msgBus, pairingSvc, agentStore, teamStore)
	}
}

func buildChannel(name string, creds json.RawMessage, cfg json.RawMessage,
	msgBus *bus.MessageBus, pairingSvc store.PairingStore, agentStore store.AgentStore, teamStore store.TeamStore) (channels.Channel, error) {

	var c telegramCreds
	if len(creds) > 0 {
		if err := json.Unmarshal(creds, &c); err != nil {
			return nil, fmt.Errorf("decode telegram credentials: %w", err)
		}
	}
	if c.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}

	var ic telegramInstanceConfig
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &ic); err != nil {
			return nil, fmt.Errorf("decode telegram config: %w", err)
		}
	}

	// Resolve voice config: prefer the nested "voice" block.
	// When absent, promote flat legacy fields so existing DB rows need no migration.
	//
	// IMPORTANT — legacy promotion is all-or-nothing:
	// if Voice.AgentID is already set in the nested block, we assume the row
	// has been fully migrated and skip ALL flat fields.  Partial migrations
	// (nested AgentID + flat keywords) are not supported.  Migrate all voice
	// fields to the nested block in one atomic DB update.
	voiceCfg := ic.Voice
	if voiceCfg.AgentID == "" && ic.LegacyVoiceAgentID != "" {
		// Promote all flat voice fields as a unit (all-or-nothing).
		voiceCfg.AgentID = ic.LegacyVoiceAgentID
		voiceCfg.StartMessage = ic.LegacyVoiceStartMessage
		voiceCfg.IntentKeywords = ic.LegacyVoiceIntentKeywords
		voiceCfg.AffinityClearKeywords = ic.LegacyVoiceAffinityClearKeywords
		voiceCfg.AffinityTTLMinutes = ic.LegacyVoiceAffinityTTLMinutes
		voiceCfg.DMContextTemplate = ic.LegacyVoiceDMContextTemplate
		voiceCfg.AudioGuardFallbackTranscript = ic.LegacyAudioGuardFallbackTranscript
		voiceCfg.AudioGuardFallbackNoTranscript = ic.LegacyAudioGuardFallbackNoTranscript
	}
	// STT fields are batched together: if no URL, the other STT fields are meaningless.
	if voiceCfg.STTProxyURL == "" && ic.LegacySTTProxyURL != "" {
		voiceCfg.STTProxyURL = ic.LegacySTTProxyURL
		voiceCfg.STTAPIKey = ic.LegacySTTAPIKey
		voiceCfg.STTTenantID = ic.LegacySTTTenantID
		voiceCfg.STTTimeoutSeconds = ic.LegacySTTTimeoutSec
	}

	tgCfg := config.TelegramConfig{
		Enabled:        true,
		Token:          c.Token,
		Proxy:          c.Proxy,
		AllowFrom:      ic.AllowFrom,
		DMPolicy:       ic.DMPolicy,
		GroupPolicy:    ic.GroupPolicy,
		RequireMention: ic.RequireMention,
		HistoryLimit:   ic.HistoryLimit,
		StreamMode:     ic.StreamMode,
		ReactionLevel:  ic.ReactionLevel,
		MediaMaxBytes:  ic.MediaMaxBytes,
		LinkPreview:    ic.LinkPreview,
		Voice:          voiceCfg,
	}

	// DB instances default to "pairing" for groups (secure by default).
	// Config-based channels keep "open" default for backward compat.
	if tgCfg.GroupPolicy == "" {
		tgCfg.GroupPolicy = "pairing"
	}

	ch, err := New(tgCfg, msgBus, pairingSvc, agentStore, teamStore)
	if err != nil {
		return nil, err
	}

	// Override the channel name from DB instance.
	ch.SetName(name)
	return ch, nil
}
