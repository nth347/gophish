package models

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestTelegramSubmittedCredentials verifies the username/password inclusion
// toggles and that the time field renders in the host's local timezone.
func TestTelegramSubmittedCredentials(t *testing.T) {
	det, _ := json.Marshal(EventDetails{
		Payload: url.Values{
			"email":    {"victim@corp.com"},
			"password": {"P@ssw0rd!"},
			"rid":      {"AbC1234"},
		},
	})
	e := &Event{
		Message:    EventDataSubmit,
		Email:      "victim@corp.com",
		CampaignId: 1,
		Time:       time.Now().UTC(),
		Details:    string(det),
	}

	// Default: neither credential included.
	msg := (&Webhook{}).formatTelegramMessage(e)
	if strings.Contains(msg, "Username:") || strings.Contains(msg, "Password:") {
		t.Errorf("default message should not include credentials, got:\n%s", msg)
	}

	// Username only.
	msg = (&Webhook{TelegramIncludeUsername: true}).formatTelegramMessage(e)
	if !strings.Contains(msg, "Username: victim@corp.com") {
		t.Errorf("expected username line, got:\n%s", msg)
	}
	if strings.Contains(msg, "Password:") {
		t.Errorf("did not expect password line, got:\n%s", msg)
	}

	// Both.
	msg = (&Webhook{TelegramIncludeUsername: true, TelegramIncludePassword: true}).formatTelegramMessage(e)
	if !strings.Contains(msg, "Username: victim@corp.com") || !strings.Contains(msg, "Password: P@ssw0rd!") {
		t.Errorf("expected username and password lines, got:\n%s", msg)
	}

	// Time should be rendered in local time, matching the event's local zone.
	wantZone := e.Time.Local().Format("MST")
	if !strings.Contains(msg, "Time: "+e.Time.Local().Format("2006-01-02 15:04:05 MST")) {
		t.Errorf("expected local time (%s) in message, got:\n%s", wantZone, msg)
	}

	// Credentials are only included for Submitted Data events.
	clicked := &Event{Message: EventClicked, Email: "victim@corp.com", CampaignId: 1, Time: time.Now().UTC(), Details: string(det)}
	msg = (&Webhook{TelegramIncludeUsername: true, TelegramIncludePassword: true}).formatTelegramMessage(clicked)
	if strings.Contains(msg, "Username:") || strings.Contains(msg, "Password:") {
		t.Errorf("non-submit events should not include credentials, got:\n%s", msg)
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
