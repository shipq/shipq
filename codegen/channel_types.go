package codegen

// SerializedChannelInfo is a JSON-serializable version of channel.ChannelInfo.
// This type is used across codegen packages for channel registry information.
type SerializedChannelInfo struct {
	Name           string                     `json:"name"`
	Visibility     string                     `json:"visibility"`
	IsPublic       bool                       `json:"is_public"`
	RateLimit      *SerializedRateLimitConfig `json:"rate_limit,omitempty"`
	Messages       []SerializedMessageInfo    `json:"messages"`
	MaxRetries     int                        `json:"max_retries"`
	BackoffSeconds int                        `json:"backoff_seconds"`
	TimeoutSeconds int                        `json:"timeout_seconds"`
	RequiredRole   string                     `json:"required_role"`
	PackagePath    string                     `json:"package_path"`
	PackageName    string                     `json:"package_name"`
	// HasSetup is true when the channel package exports a function with the
	// signature `func Setup(ctx context.Context) context.Context`. When present,
	// the generated worker code calls it before each handler invocation to
	// enrich the context with dependencies (e.g., API clients, DB connections).
	HasSetup bool `json:"has_setup"`
}

// SerializedRateLimitConfig is a JSON-serializable version of channel.RateLimitConfig.
type SerializedRateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

// SerializedMessageInfo is a JSON-serializable version of channel.MessageInfo.
type SerializedMessageInfo struct {
	Direction   string                `json:"direction"`
	TypeName    string                `json:"type_name"`
	PackagePath string                `json:"package_path,omitempty"` // import path of the package defining this type
	Fields      []SerializedFieldInfo `json:"fields"`
	IsDispatch  bool                  `json:"is_dispatch"`
	HandlerName string                `json:"handler_name"`
}
