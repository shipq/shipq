package channel

import (
	"reflect"
	"regexp"
	"strings"
)

// Direction indicates whether a message type flows from client to server or vice versa.
type Direction int

const (
	ClientToServer Direction = iota
	ServerToClient
)

// MessageDef captures a message type and its direction. Created by FromClient/FromServer.
type MessageDef struct {
	Direction Direction
	ZeroValue any
}

// FromClient tags one or more message types as flowing from client to server.
// The first type in the list is treated as the "dispatch" message that triggers
// the channel handler; subsequent types are follow-up messages the client may
// send during the conversation.
func FromClient(types ...any) []MessageDef {
	defs := make([]MessageDef, len(types))
	for i, t := range types {
		defs[i] = MessageDef{Direction: ClientToServer, ZeroValue: t}
	}
	return defs
}

// FromServer tags one or more message types as flowing from server to client.
func FromServer(types ...any) []MessageDef {
	defs := make([]MessageDef, len(types))
	for i, t := range types {
		defs[i] = MessageDef{Direction: ServerToClient, ZeroValue: t}
	}
	return defs
}

// Visibility controls whether a channel is exposed to the frontend (public HTTP routes)
// or only used internally between backend services.
type Visibility int

const (
	Frontend Visibility = iota
	Backend
)

// RateLimitConfig specifies rate limiting for public channels.
type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
}

// FieldInfo represents a single field in a message struct.
type FieldInfo struct {
	Name         string             // Go field name
	Type         string             // Go type string (e.g., "string", "int64", "*time.Time")
	JSONName     string             // JSON field name from `json` tag
	JSONOmit     bool               // true if `json:"-"` or has omitempty
	Required     bool               // true if not omitempty, not pointer, not slice/map
	Tags         map[string]string  // all struct tags
	StructFields *MessageStructInfo // non-nil if the field is itself a struct
}

// MessageStructInfo represents a message struct's metadata (analogous to handler.StructInfo).
type MessageStructInfo struct {
	Name    string
	Package string
	Fields  []FieldInfo
}

// MessageInfo holds extracted metadata about a single message type registered
// with a channel. This is used by the code generator.
type MessageInfo struct {
	Direction   Direction
	TypeName    string
	Package     string
	Fields      []FieldInfo
	IsDispatch  bool   // true for the first FromClient type (the trigger message)
	HandlerName string // default: "Handle" + TypeName (e.g., HandleChatRequest for ChatRequest)
}

// ChannelInfo holds all metadata about a registered channel.
type ChannelInfo struct {
	Name           string
	Visibility     Visibility
	IsPublic       bool
	RateLimit      *RateLimitConfig
	Messages       []MessageInfo
	MaxRetries     int
	BackoffSeconds int
	TimeoutSeconds int
	RequiredRole   string
}

// App is a registration shim that captures channel metadata.
// It is NOT a runtime server -- it exists purely to collect
// information for code generation.
type App struct {
	Channels []ChannelInfo
}

// NewApp creates a new App for channel registration.
func NewApp() *App {
	return &App{
		Channels: make([]ChannelInfo, 0),
	}
}

// DefineChannel registers a frontend-visible channel.
func (a *App) DefineChannel(name string, clientMsgs []MessageDef, serverMsgs []MessageDef) *ChannelBuilder {
	return a.define(name, Frontend, clientMsgs, serverMsgs)
}

// DefineBackendChannel registers a backend-only channel (not exposed via public HTTP routes).
func (a *App) DefineBackendChannel(name string, clientMsgs []MessageDef, serverMsgs []MessageDef) *ChannelBuilder {
	return a.define(name, Backend, clientMsgs, serverMsgs)
}

func (a *App) define(name string, vis Visibility, clientMsgs []MessageDef, serverMsgs []MessageDef) *ChannelBuilder {
	info := ChannelInfo{
		Name:       name,
		Visibility: vis,
		Messages:   make([]MessageInfo, 0, len(clientMsgs)+len(serverMsgs)),
	}

	// Extract client messages. The first one is the dispatch message.
	//
	// Handler naming convention: each client_to_server message type gets a
	// default HandlerName of "Handle" + TypeName (e.g., ChatRequest →
	// HandleChatRequest). This matches the convention expected by static
	// analysis in channel_static_analysis.go, which scans for exported
	// functions named Handle<TypeName>(ctx context.Context, req *<Type>) error.
	//
	// The static analysis pass may later override HandlerName with the actual
	// function name found in the channel package source files. This default
	// ensures the generated worker code references an exported function even
	// if static analysis hasn't run yet.
	for i, msg := range clientMsgs {
		mi := extractMessageInfo(msg)
		if i == 0 {
			mi.IsDispatch = true
			mi.HandlerName = "Handle" + mi.TypeName
		}
		info.Messages = append(info.Messages, mi)
	}

	// Extract server messages.
	for _, msg := range serverMsgs {
		mi := extractMessageInfo(msg)
		info.Messages = append(info.Messages, mi)
	}

	a.Channels = append(a.Channels, info)
	return &ChannelBuilder{app: a, index: len(a.Channels) - 1}
}

// ChannelBuilder allows chaining configuration methods after defining a channel.
type ChannelBuilder struct {
	app   *App
	index int
}

// Retries sets the maximum number of retries for the channel's task.
func (cb *ChannelBuilder) Retries(n int) *ChannelBuilder {
	cb.app.Channels[cb.index].MaxRetries = n
	return cb
}

// BackoffSeconds sets the initial backoff duration in seconds for retries.
func (cb *ChannelBuilder) BackoffSeconds(s int) *ChannelBuilder {
	cb.app.Channels[cb.index].BackoffSeconds = s
	return cb
}

// TimeoutSeconds sets the task timeout in seconds.
func (cb *ChannelBuilder) TimeoutSeconds(s int) *ChannelBuilder {
	cb.app.Channels[cb.index].TimeoutSeconds = s
	return cb
}

// RequireRole marks this channel as requiring a specific RBAC role.
func (cb *ChannelBuilder) RequireRole(action string) *ChannelBuilder {
	cb.app.Channels[cb.index].RequiredRole = action
	return cb
}

// Public marks this channel as publicly accessible (no auth required)
// and applies the given rate limit configuration.
func (cb *ChannelBuilder) Public(rl RateLimitConfig) *ChannelBuilder {
	cb.app.Channels[cb.index].IsPublic = true
	cb.app.Channels[cb.index].RateLimit = &rl
	return cb
}

// extractMessageInfo uses reflection to pull metadata from a MessageDef's ZeroValue.
func extractMessageInfo(msg MessageDef) MessageInfo {
	t := reflect.TypeOf(msg.ZeroValue)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	mi := MessageInfo{
		Direction: msg.Direction,
		TypeName:  t.Name(),
		Package:   t.PkgPath(),
		Fields:    make([]FieldInfo, 0),
	}

	if t.Kind() == reflect.Struct {
		mi.Fields = extractFields(t)
	}

	return mi
}

// extractFields extracts field metadata from a struct type via reflection.
// Mirrors the pattern in handler/app.go extractStructInfo().
func extractFields(t reflect.Type) []FieldInfo {
	fields := make([]FieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fi := FieldInfo{
			Name: field.Name,
			Type: typeToString(field.Type),
			Tags: make(map[string]string),
		}

		// Parse JSON tag
		if jsonTag, ok := field.Tag.Lookup("json"); ok {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == "-" {
				fi.JSONOmit = true
				fi.JSONName = ""
			} else {
				fi.JSONName = parts[0]
				for _, opt := range parts[1:] {
					if opt == "omitempty" {
						fi.JSONOmit = true
					}
				}
			}
		} else {
			fi.JSONName = field.Name
		}

		// Determine if required: not omitempty, not a pointer, not a slice/map
		fi.Required = !fi.JSONOmit &&
			field.Type.Kind() != reflect.Ptr &&
			field.Type.Kind() != reflect.Slice &&
			field.Type.Kind() != reflect.Map

		// Store all tags for extensibility
		tagStr := string(field.Tag)
		tagRegex := regexp.MustCompile(`(\w+):"([^"]*)"`)
		matches := tagRegex.FindAllStringSubmatch(tagStr, -1)
		for _, match := range matches {
			fi.Tags[match[1]] = match[2]
		}

		// If the field is a struct (or ptr/slice of struct), recurse.
		if st := underlyingStructType(field.Type); st != nil {
			fi.StructFields = extractStructInfoForMessage(st)
		}

		fields = append(fields, fi)
	}
	return fields
}

// extractStructInfoForMessage recursively extracts struct info for nested message fields.
func extractStructInfoForMessage(t reflect.Type) *MessageStructInfo {
	return &MessageStructInfo{
		Name:    t.Name(),
		Package: t.PkgPath(),
		Fields:  extractFields(t),
	}
}

// underlyingStructType peels away pointer and slice wrappers to find the
// innermost struct type. Returns nil if the underlying type is not a struct.
func underlyingStructType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		// Skip well-known standard library types.
		if t.PkgPath() == "time" && t.Name() == "Time" {
			return nil
		}
		return t
	}
	return nil
}

// typeToString converts a reflect.Type to a string representation.
func typeToString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + typeToString(t.Elem())
	case reflect.Slice:
		return "[]" + typeToString(t.Elem())
	case reflect.Map:
		return "map[" + typeToString(t.Key()) + "]" + typeToString(t.Elem())
	default:
		if t.PkgPath() != "" {
			return t.PkgPath() + "." + t.Name()
		}
		return t.Name()
	}
}
