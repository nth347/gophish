package models

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

// makeSubmitEvent is a helper that creates a Submitted Data event with the
// given payload.
func makeSubmitEvent(payload url.Values) *Event {
	det, _ := json.Marshal(EventDetails{Payload: payload})
	return &Event{
		Message:    EventDataSubmit,
		Email:      "victim@corp.com",
		CampaignId: 1,
		Time:       time.Now().UTC(),
		Details:    string(det),
	}
}

// TestTelegramSkipWhenNothingValid verifies that a Submitted Data notification
// is skipped (returns "") when nothing passes validation.
func TestTelegramSkipWhenNothingValid(t *testing.T) {
	// Payload with short password and no tokens.
	e := makeSubmitEvent(url.Values{
		"email":    {"victim@corp.com"},
		"password": {"x"}, // too short
	})
	wh := &Webhook{
		TelegramIncludeUsername:   true,
		TelegramIncludePassword:   true,
		TelegramMinPasswordLength: 4,
	}
	msg := wh.formatTelegramMessage(e)
	if msg != "" {
		t.Errorf("expected empty message when no valid credentials, got:\n%s", msg)
	}
}

// TestTelegramCredentialsWithMinLength verifies that the min password and
// min token length filters work. Both username and password toggles must be
// on for credentials to count.
func TestTelegramCredentialsWithMinLength(t *testing.T) {
	e := makeSubmitEvent(url.Values{
		"email":    {"victim@corp.com"},
		"password": {"ab"},
		"tokens":   {"short"},
	})

	// Password too short, tokens too short → skip.
	wh := &Webhook{
		TelegramIncludeUsername:   true,
		TelegramIncludePassword:   true,
		TelegramIncludeTokens:     true,
		TelegramMinPasswordLength: 4,
		TelegramMinTokenLength:    10,
	}
	msg := wh.formatTelegramMessage(e)
	if msg != "" {
		t.Errorf("expected skip when both below minimum, got:\n%s", msg)
	}

	// Password meets min but tokens don't → send with credentials only.
	wh.TelegramMinPasswordLength = 2
	msg = wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Username: victim@corp.com") || !strings.Contains(msg, "Password: ab") {
		t.Errorf("expected username and password included, got:\n%s", msg)
	}
	if strings.Contains(msg, "Tokens:") {
		t.Errorf("did not expect tokens (below min), got:\n%s", msg)
	}
}

// TestTelegramTokens verifies the three token states: included, not-shown note,
// and token-only send (no credential toggles).
func TestTelegramTokens(t *testing.T) {
	// Full payload with creds + tokens.
	e := makeSubmitEvent(url.Values{
		"email":    {"victim@corp.com"},
		"password": {"P@ssw0rd!"},
		"tokens":   {`[{"name":"ESTSAUTH","value":"abc"}]`},
	})

	// Tokens included (credentials toggles on so hasCredentials=true).
	wh := &Webhook{TelegramIncludeUsername: true, TelegramIncludePassword: true, TelegramIncludeTokens: true}
	msg := wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Tokens: ") {
		t.Errorf("expected tokens line, got:\n%s", msg)
	}
	if strings.Contains(msg, "not shown") {
		t.Errorf("did not expect 'not shown' note, got:\n%s", msg)
	}

	// Tokens captured but toggle off → note (credentials still valid).
	wh = &Webhook{TelegramIncludeUsername: true, TelegramIncludePassword: true, TelegramIncludeTokens: false}
	msg = wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "🍪 Tokens captured (not shown)") {
		t.Errorf("expected capture note, got:\n%s", msg)
	}
	if strings.Contains(msg, "Tokens: ") {
		t.Errorf("did not expect raw tokens, got:\n%s", msg)
	}

	// Tokens only (no credential toggles, token toggle on) → sends with token content.
	wh = &Webhook{TelegramIncludeTokens: true}
	msg = wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Tokens: ") {
		t.Errorf("expected token notification for token-only capture, got:\n%s", msg)
	}
}

// TestTelegramUsernamePattern verifies the regex pattern filter for usernames.
func TestTelegramUsernamePattern(t *testing.T) {
	e := makeSubmitEvent(url.Values{
		"email":    {"victim@corp.com"},
		"password": {"P@ssw0rd!"},
	})

	// Empty pattern → accept all (both toggles on).
	wh := &Webhook{TelegramIncludeUsername: true, TelegramIncludePassword: true, TelegramUsernamePattern: ""}
	msg := wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Username: victim@corp.com") {
		t.Errorf("empty pattern should accept all, got:\n%s", msg)
	}

	// Matching pattern.
	wh.TelegramUsernamePattern = `@corp\.com$`
	msg = wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Username: victim@corp.com") {
		t.Errorf("matching pattern should include username, got:\n%s", msg)
	}

	// Non-matching pattern → username skipped. Both toggles on so
	// hasCredentials requires both. Username failed → hasCredentials=false.
	// No tokens → notification skipped entirely.
	wh.TelegramUsernamePattern = `@other\.com$`
	msg = wh.formatTelegramMessage(e)
	if msg != "" {
		t.Errorf("expected skip when username fails pattern (no valid creds), got:\n%s", msg)
	}
}

// TestTelegramTokenOnlySend verifies that tokens alone trigger a notification
// when they meet the minimum length, even without credential toggles.
func TestTelegramTokenOnlySend(t *testing.T) {
	e := makeSubmitEvent(url.Values{
		"tokens": {`[{"name":"ESTSAUTH","value":"abc"}]`},
	})

	// Token toggle on → sends with token content.
	wh := &Webhook{TelegramIncludeTokens: true}
	msg := wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "Tokens: ") {
		t.Errorf("expected token notification for token-only capture, got:\n%s", msg)
	}

	// Token toggle off but tokens captured → sends with "not shown" note.
	wh = &Webhook{TelegramIncludeTokens: false}
	msg = wh.formatTelegramMessage(e)
	if !strings.Contains(msg, "🍪 Tokens captured (not shown)") {
		t.Errorf("expected capture note even when toggle off, got:\n%s", msg)
	}

	// Tokens below min length → skip.
	wh = &Webhook{TelegramIncludeTokens: true, TelegramMinTokenLength: 10000}
	msg = wh.formatTelegramMessage(e)
	if msg != "" {
		t.Errorf("expected skip when tokens below min length, got:\n%s", msg)
	}

	// No tokens in payload → skip (no credentials either).
	eNoTokens := makeSubmitEvent(url.Values{"email": {"victim@corp.com"}})
	wh = &Webhook{TelegramIncludeTokens: true}
	msg = wh.formatTelegramMessage(eNoTokens)
	if msg != "" {
		t.Errorf("expected skip when no tokens in payload, got:\n%s", msg)
	}
}

// TestWebhookHandlesEvent verifies the per-event filtering logic for webhooks.
func TestWebhookHandlesEvent(t *testing.T) {
	cases := []struct {
		name    string
		events  string
		message string
		want    bool
	}{
		{"empty filter sends all (sent)", "", EventSent, true},
		{"empty filter sends all (submitted)", "", EventDataSubmit, true},
		{"submitted only - submitted", "submitted", EventDataSubmit, true},
		{"submitted only - clicked blocked", "submitted", EventClicked, false},
		{"submitted only - sent blocked", "submitted", EventSent, false},
		{"multiple - clicked allowed", "clicked,submitted", EventClicked, true},
		{"multiple - opened blocked", "clicked,submitted", EventOpened, false},
		{"spaces tolerated", " submitted , reported ", EventReported, true},
		{"reported", "reported", EventReported, true},
		{"unknown message with filter blocked", "submitted", "Campaign Created", false},
		{"unknown message no filter allowed", "", "Campaign Created", true},
	}
	for _, c := range cases {
		wh := Webhook{Events: c.events}
		if got := wh.HandlesEvent(c.message); got != c.want {
			t.Errorf("%s: HandlesEvent(%q) with events=%q = %v, want %v",
				c.name, c.message, c.events, got, c.want)
		}
	}
}

// TestWebhookValidateTelegram verifies validation rules for Telegram webhooks.
func TestWebhookValidateTelegram(t *testing.T) {
	valid := Webhook{Name: "tg", Type: WebhookTypeTelegram, TelegramBotToken: "123:abc", TelegramChatID: "-100"}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid telegram webhook, got %v", err)
	}
	noURL := Webhook{Name: "tg", Type: WebhookTypeTelegram, TelegramBotToken: "123:abc", TelegramChatID: "-100"}
	if err := noURL.Validate(); err != nil {
		t.Errorf("telegram webhook should not require URL, got %v", err)
	}
	missingToken := Webhook{Name: "tg", Type: WebhookTypeTelegram, TelegramChatID: "-100"}
	if err := missingToken.Validate(); err != ErrTelegramBotTokenNotSpecified {
		t.Errorf("expected ErrTelegramBotTokenNotSpecified, got %v", err)
	}
	missingChat := Webhook{Name: "tg", Type: WebhookTypeTelegram, TelegramBotToken: "123:abc"}
	if err := missingChat.Validate(); err != ErrTelegramChatIDNotSpecified {
		t.Errorf("expected ErrTelegramChatIDNotSpecified, got %v", err)
	}
	stdNoURL := Webhook{Name: "std", Type: WebhookTypeStandard}
	if err := stdNoURL.Validate(); err != ErrURLNotSpecified {
		t.Errorf("expected ErrURLNotSpecified, got %v", err)
	}
}
