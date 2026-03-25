package telegram

import (
	"fmt"
	"testing"

	"github.com/kgatilin/myhome/internal/config"
)

func TestResolveTarget_DiscoveryMode(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:dev",
			Routes:        nil, // no routes = discovery mode
		},
	}

	msg := &Message{
		Chat: Chat{ID: 12345},
		From: User{ID: 999},
	}

	target, ok := bot.resolveTarget(msg)
	if !ok {
		t.Fatal("expected message to be accepted in discovery mode")
	}
	if target != "agent:dev" {
		t.Errorf("expected target 'agent:dev', got %q", target)
	}
}

func TestResolveTarget_RestrictedMode_MatchingChat(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:dev",
			Routes: []config.TelegramRoute{
				{ChatID: 100, Target: "agent:ops"},
				{ChatID: 200, Target: "agent:dev"},
			},
		},
	}

	msg := &Message{
		Chat: Chat{ID: 100},
		From: User{ID: 999},
	}

	target, ok := bot.resolveTarget(msg)
	if !ok {
		t.Fatal("expected message from configured chat to be accepted")
	}
	if target != "agent:ops" {
		t.Errorf("expected target 'agent:ops', got %q", target)
	}
}

func TestResolveTarget_RestrictedMode_UnknownChat(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:dev",
			Routes: []config.TelegramRoute{
				{ChatID: 100, Target: "agent:ops"},
			},
		},
	}

	msg := &Message{
		Chat: Chat{ID: 999},
		From: User{ID: 1},
	}

	_, ok := bot.resolveTarget(msg)
	if ok {
		t.Error("expected message from unknown chat to be rejected in restricted mode")
	}
}

func TestResolveTarget_OwnerOnly_Owner(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:dev",
			OwnerID:       42,
			Routes: []config.TelegramRoute{
				{ChatID: 100, Target: "agent:ops", OwnerOnly: true},
			},
		},
	}

	msg := &Message{
		Chat: Chat{ID: 100},
		From: User{ID: 42},
	}

	target, ok := bot.resolveTarget(msg)
	if !ok {
		t.Fatal("expected message from owner to be accepted")
	}
	if target != "agent:ops" {
		t.Errorf("expected target 'agent:ops', got %q", target)
	}
}

func TestResolveTarget_OwnerOnly_NonOwner(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:dev",
			OwnerID:       42,
			Routes: []config.TelegramRoute{
				{ChatID: 100, Target: "agent:ops", OwnerOnly: true},
			},
		},
	}

	msg := &Message{
		Chat: Chat{ID: 100},
		From: User{ID: 99},
	}

	_, ok := bot.resolveTarget(msg)
	if ok {
		t.Error("expected message from non-owner to be rejected on owner_only route")
	}
}

func TestResolveTarget_RouteWithoutTarget_UsesDefault(t *testing.T) {
	bot := &Bot{
		cfg: &config.TelegramAdapterConfig{
			DefaultTarget: "agent:default",
			Routes: []config.TelegramRoute{
				{ChatID: 100, Target: ""},
			},
		},
	}

	msg := &Message{
		Chat: Chat{ID: 100},
		From: User{ID: 1},
	}

	target, ok := bot.resolveTarget(msg)
	if !ok {
		t.Fatal("expected message to be accepted")
	}
	if target != "agent:default" {
		t.Errorf("expected default target, got %q", target)
	}
}

func TestParseChatIDFromTarget(t *testing.T) {
	tests := []struct {
		target  string
		want    int64
		wantErr bool
	}{
		{"telegram:12345", 12345, false},
		{"telegram:-100123", -100123, false},
		{"github:repo", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got, err := parseChatIDFromTarget(tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseChatIDFromTarget(%q) error = %v, wantErr %v", tt.target, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseChatIDFromTarget(%q) = %d, want %d", tt.target, got, tt.want)
			}
		})
	}
}

func TestExtractReplyText(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{"string payload", "hello", "hello"},
		{"map with text", map[string]any{"text": "hi"}, "hi"},
		{"map with task", map[string]any{"task": "do thing"}, "do thing"},
		{"nil payload", nil, ""},
		{"empty map", map[string]any{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReplyText(tt.payload)
			if got != tt.want {
				t.Errorf("extractReplyText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfoTextFormat(t *testing.T) {
	msg := &Message{
		Chat: Chat{ID: 12345, Type: "private", Username: "testuser"},
		From: User{ID: 67890, Username: "testuser"},
	}

	expected := fmt.Sprintf("chat_id: %d\nuser_id: %d\nchat_type: %s\nusername: %s",
		msg.Chat.ID, msg.From.ID, msg.Chat.Type, msg.From.Username)

	if expected != "chat_id: 12345\nuser_id: 67890\nchat_type: private\nusername: testuser" {
		t.Errorf("unexpected info text: %s", expected)
	}
}

func TestNewBot(t *testing.T) {
	cfg := &config.TelegramAdapterConfig{
		Token:         "test-token",
		BusSocket:     "/tmp/test.sock",
		DefaultTarget: "agent:dev",
		OwnerID:       42,
		Routes: []config.TelegramRoute{
			{ChatID: 100, Target: "agent:ops", OwnerOnly: true},
		},
	}

	bus := NewBusClient(cfg.BusSocket)
	bot := NewBot(cfg, bus)

	if bot == nil {
		t.Fatal("NewBot returned nil")
	}
	if bot.cfg != cfg {
		t.Error("cfg not set")
	}
}
