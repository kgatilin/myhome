// Package telegram implements a Telegram Bot API adapter that routes messages
// from Telegram chats to agents on the deskd message bus.
package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kgatilin/myhome/internal/config"
)

// Update represents a Telegram Bot API Update object.
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message represents a Telegram Bot API Message object.
type Message struct {
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// Bot is the Telegram polling adapter.
type Bot struct {
	cfg    *config.TelegramAdapterConfig
	bus    *BusClient
	client *http.Client
	offset int
}

// NewBot creates a new Telegram bot adapter.
func NewBot(cfg *config.TelegramAdapterConfig, bus *BusClient) *Bot {
	return &Bot{
		cfg:    cfg,
		bus:    bus,
		client: &http.Client{Timeout: 35 * time.Second},
	}
}

// Run starts the bot polling loop. Blocks until an error occurs.
func (b *Bot) Run() error {
	token, err := b.resolveToken()
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}
	b.cfg.Token = token

	if err := b.bus.Connect(); err != nil {
		log.Printf("Warning: bus connect failed: %v (running without bus — /info commands still work)", err)
	} else {
		defer b.bus.Close()
	}

	// Forward bus replies to Telegram.
	go b.replyLoop()

	mode := "discovery"
	if len(b.cfg.Routes) > 0 {
		mode = fmt.Sprintf("restricted (%d routes)", len(b.cfg.Routes))
	}
	log.Printf("telegram adapter started, mode=%s, socket=%s", mode, b.cfg.BusSocket)

	for {
		updates, err := b.getUpdates()
		if err != nil {
			log.Printf("getUpdates error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, u := range updates {
			b.handleUpdate(u)
		}
	}
}

// resolveToken resolves the bot token. Checks in order:
// 1. TELEGRAM_BOT_TOKEN env var
// 2. vault:// prefix in config (calls myhome vault get)
// 3. raw token string from config
func (b *Bot) resolveToken() (string, error) {
	// Check env var first
	if envToken := os.Getenv("TELEGRAM_BOT_TOKEN"); envToken != "" {
		return envToken, nil
	}

	token := b.cfg.Token
	if token == "" {
		return "", fmt.Errorf("telegram bot token not configured (set TELEGRAM_BOT_TOKEN env, vault:// in config, or --token flag)")
	}

	if strings.HasPrefix(token, "vault://") {
		key := strings.TrimPrefix(token, "vault://")
		out, err := exec.Command("myhome", "vault", "get", key).Output()
		if err != nil {
			return "", fmt.Errorf("vault get %s: %w", key, err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	return token, nil
}

// getUpdates calls the Telegram Bot API getUpdates endpoint with long polling.
func (b *Bot) getUpdates() ([]Update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", b.cfg.Token, b.offset)
	resp, err := b.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}

	if len(result.Result) > 0 {
		b.offset = result.Result[len(result.Result)-1].UpdateID + 1
	}

	return result.Result, nil
}

// handleUpdate processes a single Telegram update.
func (b *Bot) handleUpdate(u Update) {
	if u.Message == nil || u.Message.Text == "" {
		return
	}

	msg := u.Message

	// Ignore messages from the bot itself to prevent reply loops.
	if b.cfg.BotUsername != "" && msg.From.Username == b.cfg.BotUsername {
		return
	}

	// /info always works regardless of routing.
	if msg.Text == "/info" {
		b.handleInfo(msg)
		return
	}

	// /help always works regardless of routing.
	if msg.Text == "/help" {
		b.handleHelp(msg)
		return
	}

	// Check routing.
	target, ok := b.resolveTarget(msg)
	if !ok {
		return // silently ignore unlisted chats in restricted mode
	}

	// Post to bus.
	busMsg := BusMessage{
		Type:    "message",
		ID:      fmt.Sprintf("telegram-%d-%d-%d", msg.Chat.ID, msg.MessageID, time.Now().UnixMilli()),
		Source:  fmt.Sprintf("telegram:%d", msg.Chat.ID),
		Target:  target,
		ReplyTo: fmt.Sprintf("telegram:%d", msg.Chat.ID),
		Payload: map[string]any{
			"task":     msg.Text,
			"chat_id":  msg.Chat.ID,
			"user_id":  msg.From.ID,
			"username": msg.From.Username,
		},
		Metadata: map[string]any{
			"priority": 5,
		},
	}

	if err := b.bus.Publish(busMsg); err != nil {
		log.Printf("bus publish error: %v", err)
		return
	}
	log.Printf("posted message from chat %d (user %s) to %s", msg.Chat.ID, msg.From.Username, target)
}

// resolveTarget determines which agent should receive the message.
// Returns the target and whether the message should be processed.
func (b *Bot) resolveTarget(msg *Message) (string, bool) {
	// Discovery mode: no routes configured, accept all chats.
	if len(b.cfg.Routes) == 0 {
		return b.cfg.DefaultTarget, true
	}

	// Restricted mode: only process messages from configured chats.
	for _, route := range b.cfg.Routes {
		if route.ChatID == msg.Chat.ID {
			if route.OwnerOnly && msg.From.ID != b.cfg.OwnerID {
				return "", false
			}
			if route.MentionOnly && b.cfg.BotUsername != "" && !strings.Contains(msg.Text, "@"+b.cfg.BotUsername) {
				return "", false
			}
			target := route.Target
			if target == "" {
				target = b.cfg.DefaultTarget
			}
			return target, true
		}
	}

	return "", false
}

// handleInfo responds with chat and user information.
func (b *Bot) handleInfo(msg *Message) {
	text := fmt.Sprintf("chat_id: %d\nuser_id: %d\nchat_type: %s\nusername: %s",
		msg.Chat.ID, msg.From.ID, msg.Chat.Type, msg.From.Username)
	if err := b.sendMessage(msg.Chat.ID, text); err != nil {
		log.Printf("send /info reply: %v", err)
	}
}

// handleHelp responds with available commands.
func (b *Bot) handleHelp(msg *Message) {
	text := "Available commands:\n/info — show chat_id, user_id, chat_type, username\n/help — list available commands\n\nAll other messages are routed to agents via the message bus."
	if err := b.sendMessage(msg.Chat.ID, text); err != nil {
		log.Printf("send /help reply: %v", err)
	}
}

// sendMessage sends a text message to a Telegram chat with HTML formatting.
func (b *Bot) sendMessage(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.cfg.Token)

	payload := map[string]any{
		"chat_id":    chatID,
		"text":       markdownToTelegramHTML(text),
		"parse_mode": "HTML",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendMessage payload: %w", err)
	}

	resp, err := b.client.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("sendMessage http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendMessage status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// replyLoop listens for bus replies and sends them back to Telegram.
func (b *Bot) replyLoop() {
	for reply := range b.bus.Replies() {
		// Extract chat_id from source field (format: "telegram:<chat_id>")
		// Reply target should match the original source.
		chatID, err := parseChatIDFromTarget(reply.Target)
		if err != nil {
			log.Printf("reply: cannot parse chat_id from target %q: %v", reply.Target, err)
			continue
		}

		text := extractReplyText(reply.Payload)
		if text == "" {
			log.Printf("reply: empty text extracted from payload: %+v (type: %T)", reply.Payload, reply.Payload)
			continue
		}
		log.Printf("reply: sending to chat %d: %s", chatID, text[:min(len(text), 80)])

		if err := b.sendMessage(chatID, text); err != nil {
			log.Printf("send reply to chat %d: %v", chatID, err)
		}
	}
}

// parseChatIDFromTarget extracts chat_id from a target like "telegram:12345".
func parseChatIDFromTarget(target string) (int64, error) {
	if !strings.HasPrefix(target, "telegram:") {
		return 0, fmt.Errorf("not a telegram target: %s", target)
	}
	var chatID int64
	_, err := fmt.Sscanf(strings.TrimPrefix(target, "telegram:"), "%d", &chatID)
	return chatID, err
}

// markdownToTelegramHTML converts common markdown to Telegram-compatible HTML.
// Handles: **bold**, *italic*, `code`, ```code blocks```, and escapes HTML entities.
func markdownToTelegramHTML(text string) string {
	// Escape HTML entities first
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// Code blocks: ```...``` → <pre>...</pre>
	for {
		start := strings.Index(text, "```")
		if start == -1 {
			break
		}
		// Skip optional language tag on same line
		contentStart := start + 3
		if nl := strings.Index(text[contentStart:], "\n"); nl != -1 && nl < 20 {
			contentStart += nl + 1
		}
		end := strings.Index(text[contentStart:], "```")
		if end == -1 {
			break
		}
		end += contentStart
		code := text[contentStart:end]
		text = text[:start] + "<pre>" + code + "</pre>" + text[end+3:]
	}

	// Inline code: `...` → <code>...</code>
	for {
		start := strings.Index(text, "`")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			break
		}
		end += start + 1
		code := text[start+1 : end]
		text = text[:start] + "<code>" + code + "</code>" + text[end+1:]
	}

	// Bold: **...** → <b>...</b>
	for {
		start := strings.Index(text, "**")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		bold := text[start+2 : end]
		text = text[:start] + "<b>" + bold + "</b>" + text[end+2:]
	}

	// Italic: *...* → <i>...</i> (but not inside words)
	for {
		start := strings.Index(text, "*")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "*")
		if end == -1 {
			break
		}
		end += start + 1
		italic := text[start+1 : end]
		text = text[:start] + "<i>" + italic + "</i>" + text[end+1:]
	}

	return text
}

// extractReplyText gets text from a reply payload.
func extractReplyText(payload any) string {
	switch p := payload.(type) {
	case string:
		return p
	case map[string]any:
		// Try common payload keys in order of likelihood
		for _, key := range []string{"result", "text", "task", "error"} {
			if text, ok := p[key].(string); ok && text != "" {
				return text
			}
		}
	}
	return ""
}
