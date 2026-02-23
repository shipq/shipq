package channelcompile

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestValidate_PublicWithRole_RejectsConflict(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:         "test_channel",
			IsPublic:     true,
			RequiredRole: "admin",
			RateLimit: &codegen.SerializedRateLimitConfig{
				RequestsPerMinute: 60,
				BurstSize:         10,
			},
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartRequest", IsDispatch: true, HandlerName: "HandleStartRequest"},
				{Direction: "server_to_client", TypeName: "StatusUpdate"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for public channel with required role, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be both public and require role") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PublicWithoutRateLimit_Rejects(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:      "test_channel",
			IsPublic:  true,
			RateLimit: nil,
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartRequest", IsDispatch: true, HandlerName: "HandleStartRequest"},
				{Direction: "server_to_client", TypeName: "StatusUpdate"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for public channel without rate limit, got nil")
	}
	if !strings.Contains(err.Error(), "public channels must have a rate limit") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NoFromClient_Rejects(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "test_channel",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "server_to_client", TypeName: "StatusUpdate"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for channel with no FromClient types, got nil")
	}
	if !strings.Contains(err.Error(), "must have at least one FromClient") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NoFromServer_Rejects(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "test_channel",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartRequest", IsDispatch: true, HandlerName: "HandleStartRequest"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for channel with no FromServer types, got nil")
	}
	if !strings.Contains(err.Error(), "must have at least one FromServer") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_DispatchWithoutHandler_Rejects(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "test_channel",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartRequest", IsDispatch: true, HandlerName: ""},
				{Direction: "server_to_client", TypeName: "StatusUpdate"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for dispatch type without handler, got nil")
	}
	if !strings.Contains(err.Error(), "has no matching handler function") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_ValidChannel_Passes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:         "email",
			Visibility:   "frontend",
			RequiredRole: "send_email",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "SendEmailRequest", IsDispatch: true, HandlerName: "HandleSendEmailRequest"},
				{Direction: "server_to_client", TypeName: "EmailProgress"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err != nil {
		t.Fatalf("expected no error for valid channel, got: %v", err)
	}
}

func TestValidate_ValidPublicChannel_Passes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:     "public_chat",
			IsPublic: true,
			RateLimit: &codegen.SerializedRateLimitConfig{
				RequestsPerMinute: 30,
				BurstSize:         5,
			},
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "ChatMessage", IsDispatch: true, HandlerName: "HandleChatMessage"},
				{Direction: "server_to_client", TypeName: "ChatResponse"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err != nil {
		t.Fatalf("expected no error for valid public channel, got: %v", err)
	}
}

func TestValidate_ValidBidirectionalChannel_Passes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "chatbot",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartChat", IsDispatch: true, HandlerName: "HandleStartChat"},
				{Direction: "client_to_server", TypeName: "ToolCallApproval", IsDispatch: false, HandlerName: "HandleToolCallApproval"},
				{Direction: "server_to_client", TypeName: "BotMessage"},
				{Direction: "server_to_client", TypeName: "ToolCallRequest"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err != nil {
		t.Fatalf("expected no error for valid bidirectional channel, got: %v", err)
	}
}

func TestValidate_MultipleErrors_ReportsAll(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:         "bad_channel",
			IsPublic:     true,
			RequiredRole: "admin",
			RateLimit:    nil,
			Messages:     []codegen.SerializedMessageInfo{},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected errors, got nil")
	}

	errStr := err.Error()

	// Should report multiple issues
	if !strings.Contains(errStr, "cannot be both public and require role") {
		t.Error("expected public+role conflict error")
	}
	if !strings.Contains(errStr, "public channels must have a rate limit") {
		t.Error("expected rate limit error")
	}
	if !strings.Contains(errStr, "must have at least one FromClient") {
		t.Error("expected FromClient error")
	}
	if !strings.Contains(errStr, "must have at least one FromServer") {
		t.Error("expected FromServer error")
	}
}

func TestValidate_EmptyChannelList_Passes(t *testing.T) {
	err := ValidateChannels([]codegen.SerializedChannelInfo{})
	if err != nil {
		t.Fatalf("expected no error for empty channel list, got: %v", err)
	}
}

func TestValidate_NilChannelList_Passes(t *testing.T) {
	err := ValidateChannels(nil)
	if err != nil {
		t.Fatalf("expected no error for nil channel list, got: %v", err)
	}
}

func TestValidate_MultipleChannels_ValidatesAll(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "good_channel",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Req", IsDispatch: true, HandlerName: "HandleReq"},
				{Direction: "server_to_client", TypeName: "Resp"},
			},
		},
		{
			Name:     "bad_channel",
			Messages: []codegen.SerializedMessageInfo{},
		},
	}

	err := ValidateChannels(channels)
	if err == nil {
		t.Fatal("expected error for bad channel, got nil")
	}
	if !strings.Contains(err.Error(), "bad_channel") {
		t.Errorf("expected error to mention bad_channel: %v", err)
	}
}

func TestValidate_BackendChannelWithRole_Passes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:         "admin_notifications",
			Visibility:   "backend",
			IsPublic:     false,
			RequiredRole: "admin",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Subscribe", IsDispatch: true, HandlerName: "HandleSubscribe"},
				{Direction: "server_to_client", TypeName: "Notification"},
			},
		},
	}

	err := ValidateChannels(channels)
	if err != nil {
		t.Fatalf("expected no error for backend channel with role, got: %v", err)
	}
}
