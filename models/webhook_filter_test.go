package models

import "testing"

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
	// Telegram webhook does not require a URL
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
	// Standard webhook still requires a URL
	stdNoURL := Webhook{Name: "std", Type: WebhookTypeStandard}
	if err := stdNoURL.Validate(); err != ErrURLNotSpecified {
		t.Errorf("expected ErrURLNotSpecified, got %v", err)
	}
}
