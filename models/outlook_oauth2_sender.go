package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
)

const (
	outlookGraphSendMailURL = "https://graph.microsoft.com/v1.0/me/sendMail"
	outlookTokenURL         = "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"
	outlookScope            = "https://graph.microsoft.com/Mail.Send offline_access"
	outlookSendTimeout      = 30 * time.Second
)

// outlookToken is the native token format stored in outlook_token_cache.
type outlookToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// OutlookOAuth2Dialer implements mailer.Dialer and mailer.HTTPSender.
type OutlookOAuth2Dialer struct {
	profile SMTP
}

func (d *OutlookOAuth2Dialer) Dial() (mailer.Sender, error) {
	return nil, fmt.Errorf("Outlook OAuth2 sending profiles do not support SMTP dialing")
}

func (d *OutlookOAuth2Dialer) BatchSize() int { return 1 }

// SendEmail delivers a single email via the Microsoft Graph API.
func (d *OutlookOAuth2Dialer) SendEmail(ctx context.Context, content *mailer.EmailContent) error {
	token, err := d.getOrRefreshToken()
	if err != nil {
		return fmt.Errorf("failed to get Outlook access token: %v", err)
	}

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"subject": content.Subject,
			"body":    map[string]string{"contentType": "HTML", "content": content.HTML},
			"toRecipients": []map[string]interface{}{
				{"emailAddress": map[string]string{"address": content.To}},
			},
			"from": map[string]interface{}{
				"emailAddress": map[string]string{"address": d.profile.FromAddress},
			},
		},
		"saveToSentItems": "true",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", outlookGraphSendMailURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: outlookSendTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		snippet := string(respBody)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return fmt.Errorf("Graph API returned status %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

func (d *OutlookOAuth2Dialer) getOrRefreshToken() (*outlookToken, error) {
	if d.profile.OutlookTokenCache == "" {
		return nil, ErrOutlookNotAuthenticated
	}
	var token outlookToken
	if err := json.Unmarshal([]byte(d.profile.OutlookTokenCache), &token); err != nil {
		return nil, fmt.Errorf("invalid Outlook token cache: %v", err)
	}
	if time.Now().Add(5 * time.Minute).Before(token.ExpiresAt) {
		return &token, nil
	}
	refreshed, err := refreshOutlookToken(d.profile.OutlookClientID, token.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Outlook token: %v", err)
	}
	if dbErr := SaveOutlookToken(d.profile.Id, refreshed); dbErr != nil {
		log.Warnf("Failed to persist refreshed Outlook token: %v", dbErr)
	}
	return refreshed, nil
}

func refreshOutlookToken(clientID, refreshToken string) (*outlookToken, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("refresh_token", refreshToken)
	data.Set("scope", outlookScope)

	resp, err := http.PostForm(outlookTokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	return &outlookToken{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

func SaveOutlookToken(smtpID int64, token *outlookToken) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return db.Model(&SMTP{}).Where("id = ?", smtpID).
		Update("outlook_token_cache", string(tokenJSON)).Error
}

// msalTokenCache mirrors the relevant parts of msal.SerializableTokenCache.
type msalTokenCache struct {
	AccessToken  map[string]msalAccessTokenEntry  `json:"AccessToken"`
	RefreshToken map[string]msalRefreshTokenEntry `json:"RefreshToken"`
}

type msalAccessTokenEntry struct {
	Secret    string `json:"secret"`
	ExpiresOn string `json:"expires_on"`
	ClientID  string `json:"client_id"`
}

type msalRefreshTokenEntry struct {
	Secret string `json:"secret"`
}

// ImportOutlookTokenCache accepts either our native JSON or an MSAL SerializableTokenCache
// and returns a normalised outlookToken ready to be stored in the DB.
func ImportOutlookTokenCache(raw string) (*outlookToken, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("token cache is empty")
	}

	var native outlookToken
	if err := json.Unmarshal([]byte(raw), &native); err == nil && native.AccessToken != "" {
		return &native, nil
	}

	var msal msalTokenCache
	if err := json.Unmarshal([]byte(raw), &msal); err != nil {
		return nil, fmt.Errorf("cannot parse token cache (expected MSAL or native JSON): %v", err)
	}

	var accessToken, refreshToken string
	var expiresAt time.Time

	for _, entry := range msal.AccessToken {
		accessToken = entry.Secret
		if entry.ExpiresOn != "" {
			ts, err := strconv.ParseInt(entry.ExpiresOn, 10, 64)
			if err == nil {
				expiresAt = time.Unix(ts, 0)
			}
		}
		break
	}
	for _, entry := range msal.RefreshToken {
		refreshToken = entry.Secret
		break
	}

	if accessToken == "" {
		return nil, fmt.Errorf("no access token found in MSAL cache — make sure you ran the script and it saved token_cache.json")
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(time.Hour)
	}

	return &outlookToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

func ExtractClientIDFromMSAL(raw string) string {
	var msal msalTokenCache
	if err := json.Unmarshal([]byte(raw), &msal); err != nil {
		return ""
	}
	for _, entry := range msal.AccessToken {
		return entry.ClientID
	}
	return ""
}
