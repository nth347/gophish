package models

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
)

// Webhook interface types
const (
	WebhookTypeStandard = "standard"
	WebhookTypeTelegram = "telegram"
)

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
		if err := webhook.SendTelegram(wh.TelegramBotToken, wh.TelegramChatID, formatTelegramMessage(e)); err != nil {
			log.Errorf("error sending telegram webhook: %v", err)
		}
		return
	}
	if err := webhook.Send(webhook.EndPoint{URL: wh.URL, Secret: wh.Secret}, e); err != nil {
		log.Errorf("error sending webhook: %v", err)
	}
}

// formatTelegramMessage builds a human-readable Telegram notification for an
// event.
func formatTelegramMessage(e *Event) string {
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
	lines = append(lines, "Time: "+e.Time.Format("2006-01-02 15:04:05 MST"))
	return strings.Join(lines, "\n")
}
