// Package voiceguard provides the Telegram voice-agent audio guard.
//
// Responsibility: intercept replies from a configured voice agent on
// Telegram DM turns that carried an audio/voice message and replace any
// technical-error language with a user-friendly coaching fallback.
//
// Design constraints:
//   - Zero dependency on the Telegram bot SDK, message bus, or scheduler.
//   - Pure string→string transformation — safe to unit-test in isolation.
//   - All deployment customisation is passed via [config.TelegramVoiceConfig];
//     the package itself holds no mutable state.
package voiceguard

import (
	"html"
	"regexp"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// transcriptTagRe matches the first <transcript>…</transcript> block,
// including multi-line content.
var transcriptTagRe = regexp.MustCompile(`(?s)<transcript>(.*?)</transcript>`)

// defaultFallbackTranscript is the built-in coaching message when the agent
// reply contains error language AND the inbound message has a transcript.
// Use strings.ReplaceAll (not fmt.Sprintf) so that custom templates that
// omit %s do not produce "%!(EXTRA string=…)" garbage.
const defaultFallbackTranscript = "🎙️ Got your voice message! I heard: \"%s\"\n\n" +
	"There was a brief hiccup on my end — please send your response again and I'll review it right away."

// defaultFallbackNoTranscript is used when no transcript is available.
const defaultFallbackNoTranscript = "🎙️ Got your voice message!\n\n" +
	"I had a little trouble processing it — could you send it again or type your response? I'll get back to you straight away."

// defaultErrorMarkers is the built-in set of substrings (all lowercase) that
// indicate a technical error leaked into the agent reply.
//
// NOTE — AudioGuardErrorMarkers in TelegramVoiceConfig REPLACES (not extends)
// this list.  When an operator sets custom markers, only those markers are
// checked; the defaults below are ignored.  To augment the defaults, copy this
// list into your config and append your custom entries.
var defaultErrorMarkers = []string{
	"vấn đề kỹ thuật",
	"vấn đề hệ thống",
	"lỗi hệ thống",
	"technical issue",
	"system error",
	"exit status",
	"rate limit",
	"api rate limit",
	"tool error",
}

// SanitizeReply intercepts replies from the configured voice agent on Telegram
// DMs and replaces any technical-error language with a user-friendly fallback.
//
// It returns the original reply unchanged when any of the following is true:
//   - voiceAgentID is empty, or agentID ≠ voiceAgentID (wrong agent)
//   - channel ≠ "telegram"
//   - peerKind ≠ "direct" (group chat)
//   - inbound contains neither <media:voice> nor <media:audio> (text-only turn)
//   - reply does not contain recognised error language
//
// Parameters:
//   - voiceAgentID: value of cfg.Channels.Telegram.Voice.AgentID
//   - agentID:      the agent that produced this reply
//   - channel:      channel transport name (e.g. "telegram")
//   - peerKind:     "direct" or "group"
//   - inbound:      original inbound message content (may contain XML-like tags)
//   - reply:        agent reply to inspect and possibly replace
//   - voiceCfg:     TelegramVoiceConfig from the channel config
func SanitizeReply(
	voiceAgentID, agentID, channel, peerKind, inbound, reply string,
	voiceCfg config.TelegramVoiceConfig,
) string {
	if voiceAgentID == "" || agentID != voiceAgentID {
		return reply
	}
	if channel != "telegram" || peerKind != "direct" {
		return reply
	}
	if !strings.Contains(inbound, "<media:voice>") && !strings.Contains(inbound, "<media:audio>") {
		return reply
	}
	if !containsErrorLanguage(reply, voiceCfg.AudioGuardErrorMarkers) {
		return reply
	}

	transcript := extractTranscript(inbound)
	if transcript != "" {
		tpl := voiceCfg.AudioGuardFallbackTranscript
		if tpl == "" {
			tpl = defaultFallbackTranscript
		}
		// strings.ReplaceAll: templates without %s pass through unchanged.
		return strings.ReplaceAll(tpl, "%s", transcript)
	}

	msg := voiceCfg.AudioGuardFallbackNoTranscript
	if msg == "" {
		msg = defaultFallbackNoTranscript
	}
	return msg
}

// containsErrorLanguage reports whether s (lowercased) contains any marker.
//
// When customMarkers is non-empty it is used exclusively — the built-in
// defaultErrorMarkers list is NOT consulted.  This is intentional: operators
// who set custom markers take full ownership of the detection set.  See the
// AudioGuardErrorMarkers field comment in TelegramVoiceConfig for the rationale.
func containsErrorLanguage(s string, customMarkers []string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return false
	}
	markers := customMarkers
	if len(markers) == 0 {
		markers = defaultErrorMarkers
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// extractTranscript returns the content of the first <transcript>…</transcript>
// block found in content, with HTML entities unescaped and whitespace collapsed.
// Returns "" when no block is present.
func extractTranscript(content string) string {
	m := transcriptTagRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	t := strings.TrimSpace(html.UnescapeString(m[1]))
	return strings.Join(strings.Fields(t), " ")
}
