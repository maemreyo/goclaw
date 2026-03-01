package config

// ChannelsConfig contains per-channel configuration.
type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
	Slack    SlackConfig    `json:"slack"`
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Zalo     ZaloConfig     `json:"zalo"`
	Feishu   FeishuConfig   `json:"feishu"`
}

// TelegramVoiceConfig groups all voice-specific settings for the Telegram channel
// under a single nested JSON key "voice".  This provides a clean visual boundary
// between base channel settings (token, policies, media) and the voice pipeline.
type TelegramVoiceConfig struct {
	// ── STT (Speech-to-Text) pipeline ─────────────────────────────────────────
	// When STTProxyURL is set, audio/voice inbound messages are transcribed before
	// being forwarded to the agent.
	STTProxyURL       string `json:"stt_proxy_url,omitempty"`       // base URL of STT proxy (e.g. "https://stt.example.com")
	STTAPIKey         string `json:"stt_api_key,omitempty"`         // Bearer token for the STT proxy
	STTTenantID       string `json:"stt_tenant_id,omitempty"`       // forwarded to STT proxy; settable via GOCLAW_STT_TENANT_ID env var
	STTTimeoutSeconds int    `json:"stt_timeout_seconds,omitempty"` // per-request timeout (default 30s)

	// ── Audio-aware routing ───────────────────────────────────────────────────
	// When AgentID is set, voice/audio inbound messages are routed to this agent
	// instead of the default channel agent.
	AgentID      string `json:"agent_id,omitempty"`      // e.g. "speaking-agent"; settable via GOCLAW_VOICE_AGENT_ID env var
	StartMessage string `json:"start_message,omitempty"` // content injected on /start; default "User sent /start."

	// ── Intent routing ────────────────────────────────────────────────────────
	// Inbound text is lowercased before matching; keywords are also lowercased at
	// match time to tolerate mixed-case values from the DB.
	// When non-empty, text messages matching any keyword are routed to AgentID
	// and the DM affinity is set.
	// Example: ["speaking", "pronunciation", "ielts part"]
	IntentKeywords []string `json:"intent_keywords,omitempty"`

	// ── Session affinity management ───────────────────────────────────────────
	// AffinityClearKeywords: when a DM text matches any entry the affinity is
	// cleared and the next message routes back to the default agent.
	// Example: ["homework", "payment", "schedule"]
	AffinityClearKeywords []string `json:"affinity_clear_keywords,omitempty"`
	// AffinityTTLMinutes: 0 = built-in default (360 min = 6 h).
	AffinityTTLMinutes int `json:"affinity_ttl_minutes,omitempty"`

	// ── DM context injection ──────────────────────────────────────────────────
	// DMContextTemplate is injected as extra system prompt on Telegram DM turns
	// handled by the voice agent.  Supports {user_id} placeholder.
	// Settable via GOCLAW_VOICE_DM_CONTEXT_TEMPLATE env var.
	//
	// Example:
	//
	//	"Context:\n- tenant: my-school\n- user_id: {user_id}\nNEVER expose errors."
	DMContextTemplate string `json:"dm_context_template,omitempty"`

	// ── Audio guard ───────────────────────────────────────────────────────────
	// Replaces technical-error agent replies with user-friendly coaching fallbacks.
	//
	//   AudioGuardFallbackTranscript:    sent when a <transcript> block is present.
	//                                    Supports %s as a placeholder for the transcript text.
	//   AudioGuardFallbackNoTranscript:  sent when no transcript is available.
	//   AudioGuardErrorMarkers:          lowercase substrings that trigger the guard.
	//
	// IMPORTANT — AudioGuardErrorMarkers REPLACES (not extends) the built-in English+
	// Vietnamese marker list.  To augment the defaults, copy the default list and append
	// your custom entries.  Leave empty to use the built-in defaults unchanged.
	AudioGuardFallbackTranscript   string   `json:"audio_guard_fallback_transcript,omitempty"`
	AudioGuardFallbackNoTranscript string   `json:"audio_guard_fallback_no_transcript,omitempty"`
	AudioGuardErrorMarkers         []string `json:"audio_guard_error_markers,omitempty"`
}

type TelegramConfig struct {
	Enabled        bool                `json:"enabled"`
	Token          string              `json:"token"`
	Proxy          string              `json:"proxy,omitempty"`
	AllowFrom      FlexibleStringSlice `json:"allow_from"`
	DMPolicy       string              `json:"dm_policy,omitempty"`       // "pairing" (default), "allowlist", "open", "disabled"
	GroupPolicy    string              `json:"group_policy,omitempty"`    // "open" (default), "allowlist", "disabled"
	RequireMention *bool               `json:"require_mention,omitempty"` // require @bot mention in groups (default true)
	HistoryLimit   int                 `json:"history_limit,omitempty"`   // max pending group messages (default 50, 0=disabled)
	StreamMode     string              `json:"stream_mode,omitempty"`     // "off" (default), "partial" — streaming via message edits
	ReactionLevel  string              `json:"reaction_level,omitempty"`  // "off" (default), "minimal", "full"
	MediaMaxBytes  int64               `json:"media_max_bytes,omitempty"` // max media download size in bytes (default 20 MB)
	LinkPreview    *bool               `json:"link_preview,omitempty"`    // enable URL previews in messages (default true)

	// Voice groups all voice-pipeline settings (STT, routing, affinity, audio guard).
	// DB rows using the older flat layout (voice_agent_id, stt_proxy_url, …) are still
	// supported — factory.go promotes flat fields into Voice on load.
	Voice TelegramVoiceConfig `json:"voice,omitempty"`
}

type DiscordConfig struct {
	Enabled        bool                `json:"enabled"`
	Token          string              `json:"token"`
	AllowFrom      FlexibleStringSlice `json:"allow_from"`
	DMPolicy       string              `json:"dm_policy,omitempty"`       // "open" (default), "allowlist", "disabled"
	GroupPolicy    string              `json:"group_policy,omitempty"`    // "open" (default), "allowlist", "disabled"
	RequireMention *bool               `json:"require_mention,omitempty"` // require @bot mention in groups (default true)
	HistoryLimit   int                 `json:"history_limit,omitempty"`   // max pending group messages for context (default 50, 0=disabled)
}

type SlackConfig struct {
	Enabled        bool                `json:"enabled"`
	BotToken       string              `json:"bot_token"`
	AppToken       string              `json:"app_token"`
	AllowFrom      FlexibleStringSlice `json:"allow_from"`
	DMPolicy       string              `json:"dm_policy,omitempty"`       // "open" (default), "allowlist", "disabled"
	GroupPolicy    string              `json:"group_policy,omitempty"`    // "open" (default), "allowlist", "disabled"
	RequireMention bool               `json:"require_mention,omitempty"` // only respond to @bot in channels (default true)
}

type WhatsAppConfig struct {
	Enabled     bool                `json:"enabled"`
	BridgeURL   string              `json:"bridge_url"`
	AllowFrom   FlexibleStringSlice `json:"allow_from"`
	DMPolicy    string              `json:"dm_policy,omitempty"`    // "open" (default), "allowlist", "disabled"
	GroupPolicy string              `json:"group_policy,omitempty"` // "open" (default), "allowlist", "disabled"
}

type ZaloConfig struct {
	Enabled       bool                `json:"enabled"`
	Token         string              `json:"token"`
	AllowFrom     FlexibleStringSlice `json:"allow_from"`
	DMPolicy      string              `json:"dm_policy,omitempty"`       // "pairing" (default), "allowlist", "open", "disabled"
	WebhookURL    string              `json:"webhook_url,omitempty"`
	WebhookSecret string              `json:"webhook_secret,omitempty"`
	MediaMaxMB    int                 `json:"media_max_mb,omitempty"` // default 5
}

type FeishuConfig struct {
	Enabled           bool                `json:"enabled"`
	AppID             string              `json:"app_id"`
	AppSecret         string              `json:"app_secret"`
	EncryptKey        string              `json:"encrypt_key,omitempty"`
	VerificationToken string              `json:"verification_token,omitempty"`
	Domain            string              `json:"domain,omitempty"`             // "lark" (default/global), "feishu" (China), or custom URL
	ConnectionMode    string              `json:"connection_mode,omitempty"`    // "websocket" (default), "webhook"
	WebhookPort       int                 `json:"webhook_port,omitempty"`       // default 3000
	WebhookPath       string              `json:"webhook_path,omitempty"`       // default "/feishu/events"
	AllowFrom         FlexibleStringSlice `json:"allow_from"`
	DMPolicy          string              `json:"dm_policy,omitempty"`          // "pairing" (default)
	GroupPolicy       string              `json:"group_policy,omitempty"`       // "open" (default)
	GroupAllowFrom    FlexibleStringSlice `json:"group_allow_from,omitempty"`
	RequireMention    *bool               `json:"require_mention,omitempty"`    // default true (groups)
	TopicSessionMode  string              `json:"topic_session_mode,omitempty"` // "disabled" (default)
	TextChunkLimit    int                 `json:"text_chunk_limit,omitempty"`   // default 4000
	MediaMaxMB        int                 `json:"media_max_mb,omitempty"`       // default 30
	RenderMode        string              `json:"render_mode,omitempty"`        // "auto", "raw", "card"
	Streaming         *bool               `json:"streaming,omitempty"`          // default true
	HistoryLimit      int                 `json:"history_limit,omitempty"`
}

// ProvidersConfig maps provider name to its config.
type ProvidersConfig struct {
	Anthropic  ProviderConfig `json:"anthropic"`
	OpenAI     ProviderConfig `json:"openai"`
	OpenRouter ProviderConfig `json:"openrouter"`
	Groq       ProviderConfig `json:"groq"`
	Gemini     ProviderConfig `json:"gemini"`
	DeepSeek   ProviderConfig `json:"deepseek"`
	Mistral    ProviderConfig `json:"mistral"`
	XAI        ProviderConfig `json:"xai"`
	MiniMax    ProviderConfig `json:"minimax"`
	Cohere     ProviderConfig `json:"cohere"`
	Perplexity ProviderConfig `json:"perplexity"`
	DashScope  ProviderConfig `json:"dashscope"`
	Bailian    ProviderConfig `json:"bailian"`
}

type ProviderConfig struct {
	APIKey  string `json:"api_key"`
	APIBase string `json:"api_base,omitempty"`
}

// HasAnyProvider returns true if at least one provider has an API key configured.
func (c *Config) HasAnyProvider() bool {
	p := c.Providers
	return p.Anthropic.APIKey != "" ||
		p.OpenAI.APIKey != "" ||
		p.OpenRouter.APIKey != "" ||
		p.Groq.APIKey != "" ||
		p.Gemini.APIKey != "" ||
		p.DeepSeek.APIKey != "" ||
		p.Mistral.APIKey != "" ||
		p.XAI.APIKey != "" ||
		p.MiniMax.APIKey != "" ||
		p.Cohere.APIKey != "" ||
		p.Perplexity.APIKey != "" ||
		p.DashScope.APIKey != "" ||
		p.Bailian.APIKey != ""
}

// GatewayConfig controls the gateway server.
type GatewayConfig struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	Token           string   `json:"token,omitempty"`              // bearer token for WS/HTTP auth
	OwnerIDs        []string `json:"owner_ids,omitempty"`          // sender IDs considered "owner"
	AllowedOrigins  []string `json:"allowed_origins,omitempty"`    // WebSocket CORS whitelist (empty = allow all)
	MaxMessageChars int      `json:"max_message_chars,omitempty"`  // max user message characters (default 32000)
	RateLimitRPM      int      `json:"rate_limit_rpm,omitempty"`       // rate limit: requests per minute per user (default 20, 0 = disabled)
	InjectionAction   string   `json:"injection_action,omitempty"`     // prompt injection action: "log", "warn" (default), "block", "off"
	InboundDebounceMs int      `json:"inbound_debounce_ms,omitempty"` // merge rapid messages from same sender (default 1000ms, -1 = disabled)
}

// ToolsConfig controls tool availability, policy, and web search.
type ToolsConfig struct {
	Profile          string                        `json:"profile,omitempty"`            // global profile: "minimal", "coding", "messaging", "full"
	Allow            []string                      `json:"allow,omitempty"`              // global allow list (tool names or "group:xxx")
	Deny             []string                      `json:"deny,omitempty"`               // global deny list
	AlsoAllow        []string                      `json:"alsoAllow,omitempty"`          // additive: adds without removing existing
	ByProvider       map[string]*ToolPolicySpec    `json:"byProvider,omitempty"`         // per-provider overrides
	ExecApproval     ExecApprovalCfg               `json:"execApproval,omitempty"`       // exec command approval settings
	Web              WebToolsConfig                `json:"web"`
	Browser          BrowserToolConfig             `json:"browser"`
	RateLimitPerHour int                           `json:"rate_limit_per_hour,omitempty"` // max tool executions per hour per session (0 = disabled)
	ScrubCredentials *bool                         `json:"scrub_credentials,omitempty"`   // auto-redact API keys/tokens in tool output (default true)
	McpServers       map[string]*MCPServerConfig   `json:"mcp_servers,omitempty"`         // external MCP server connections
}

// MCPServerConfig configures a single external MCP server connection.
type MCPServerConfig struct {
	Transport  string            `json:"transport"`               // "stdio", "sse", "streamable-http"
	Command    string            `json:"command,omitempty"`       // stdio: command to spawn
	Args       []string          `json:"args,omitempty"`          // stdio: command arguments
	Env        map[string]string `json:"env,omitempty"`           // stdio: extra environment variables
	URL        string            `json:"url,omitempty"`           // sse/http: server URL
	Headers    map[string]string `json:"headers,omitempty"`       // sse/http: extra HTTP headers
	Enabled    *bool             `json:"enabled,omitempty"`       // default true
	ToolPrefix string            `json:"tool_prefix,omitempty"`   // prefix for tool names (avoids collisions)
	TimeoutSec int               `json:"timeout_sec,omitempty"`   // per-tool-call timeout in seconds (default 60)
}

// IsEnabled returns whether this MCP server is enabled (default true).
func (c *MCPServerConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// ExecApprovalCfg configures command execution approval (matching TS exec-approval.ts).
type ExecApprovalCfg struct {
	Security  string   `json:"security,omitempty"`  // "deny", "allowlist", "full" (default "full")
	Ask       string   `json:"ask,omitempty"`       // "off", "on-miss", "always" (default "off")
	Allowlist []string `json:"allowlist,omitempty"` // glob patterns for allowed commands
}

// BrowserToolConfig controls the browser automation tool.
type BrowserToolConfig struct {
	Enabled  bool `json:"enabled"`            // enable the browser tool (default false)
	Headless bool `json:"headless,omitempty"` // run Chrome in headless mode
}

// ToolPolicySpec defines a tool policy at any level (global, per-agent, per-provider).
type ToolPolicySpec struct {
	Profile    string                     `json:"profile,omitempty"`
	Allow      []string                   `json:"allow,omitempty"`
	Deny       []string                   `json:"deny,omitempty"`
	AlsoAllow  []string                   `json:"alsoAllow,omitempty"`
	ByProvider map[string]*ToolPolicySpec `json:"byProvider,omitempty"`
	Vision     *VisionConfig              `json:"vision,omitempty"`   // per-agent vision provider/model override
	ImageGen   *ImageGenConfig            `json:"imageGen,omitempty"` // per-agent image generation config
}

// VisionConfig configures the provider and model for vision tools (read_image).
type VisionConfig struct {
	Provider string `json:"provider,omitempty"` // e.g. "gemini", "anthropic"
	Model    string `json:"model,omitempty"`    // e.g. "gemini-2.0-flash"
}

// ImageGenConfig configures the provider and model for image generation (create_image).
type ImageGenConfig struct {
	Provider string `json:"provider,omitempty"` // provider with image gen API (e.g. "openrouter")
	Model    string `json:"model,omitempty"`    // e.g. "google/gemini-2.5-flash-image-preview"
	Size     string `json:"size,omitempty"`     // default aspect ratio / size
	Quality  string `json:"quality,omitempty"`  // "standard" or "hd"
}

type WebToolsConfig struct {
	Brave      BraveConfig      `json:"brave"`
	DuckDuckGo DuckDuckGoConfig `json:"duckduckgo"`
}

type BraveConfig struct {
	Enabled    bool   `json:"enabled"`
	APIKey     string `json:"api_key"`
	MaxResults int    `json:"max_results"`
}

type DuckDuckGoConfig struct {
	Enabled    bool `json:"enabled"`
	MaxResults int  `json:"max_results"`
}

// SessionsConfig controls session behavior.
// Matching TS src/config/sessions/types.ts + src/config/types.base.ts.
type SessionsConfig struct {
	Storage string `json:"storage"`              // directory for session files
	Scope   string `json:"scope,omitempty"`      // "per-sender" (default), "global"
	DmScope string `json:"dm_scope,omitempty"`   // "main", "per-peer", "per-channel-peer" (default), "per-account-channel-peer"
	MainKey string `json:"main_key,omitempty"`   // main session key suffix (default "main", used when dm_scope="main")
}

// TtsConfig configures text-to-speech.
// Matching TS src/config/types.tts.ts.
type TtsConfig struct {
	Provider   string              `json:"provider,omitempty"`    // "openai", "elevenlabs", "edge", "minimax"
	Auto       string              `json:"auto,omitempty"`        // "off" (default), "always", "inbound", "tagged"
	Mode       string              `json:"mode,omitempty"`        // "final" (default), "all"
	MaxLength  int                 `json:"max_length,omitempty"`  // max text length before truncation (default 1500)
	TimeoutMs  int                 `json:"timeout_ms,omitempty"`  // API timeout in ms (default 30000)
	OpenAI     TtsOpenAIConfig     `json:"openai,omitempty"`
	ElevenLabs TtsElevenLabsConfig `json:"elevenlabs,omitempty"`
	Edge       TtsEdgeConfig       `json:"edge,omitempty"`
	MiniMax    TtsMiniMaxConfig    `json:"minimax,omitempty"`
}

// TtsOpenAIConfig configures the OpenAI TTS provider.
type TtsOpenAIConfig struct {
	APIKey  string `json:"api_key,omitempty"`
	APIBase string `json:"api_base,omitempty"` // custom endpoint URL
	Model   string `json:"model,omitempty"`    // default "gpt-4o-mini-tts"
	Voice   string `json:"voice,omitempty"`    // default "alloy"
}

// TtsElevenLabsConfig configures the ElevenLabs TTS provider.
type TtsElevenLabsConfig struct {
	APIKey  string `json:"api_key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	VoiceID string `json:"voice_id,omitempty"` // default "pMsXgVXv3BLzUgSXRplE"
	ModelID string `json:"model_id,omitempty"` // default "eleven_multilingual_v2"
}

// TtsEdgeConfig configures the Microsoft Edge TTS provider (free, no API key).
type TtsEdgeConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Voice   string `json:"voice,omitempty"` // default "en-US-MichelleNeural"
	Rate    string `json:"rate,omitempty"`  // speech rate, e.g. "+0%"
}

// TtsMiniMaxConfig configures the MiniMax TTS provider.
type TtsMiniMaxConfig struct {
	APIKey  string `json:"api_key,omitempty"`
	GroupID string `json:"group_id,omitempty"` // MiniMax GroupId (required)
	APIBase string `json:"api_base,omitempty"` // default "https://api.minimax.io/v1"
	Model   string `json:"model,omitempty"`    // default "speech-02-hd"
	VoiceID string `json:"voice_id,omitempty"` // default "Wise_Woman"
}
