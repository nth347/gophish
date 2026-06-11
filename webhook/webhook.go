package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/gophish/gophish/logger"
)

const (
	DefaultTimeoutSeconds  = 10
	MinHTTPStatusErrorCode = 400
	SignatureHeader        = "X-Gophish-Signature"
	Sha256Prefix           = "sha256"
)

type Sender interface {
	Send(endPoint EndPoint, data interface{}) error
}

type defaultSender struct {
	client *http.Client
}

var senderInstance = &defaultSender{
	client: &http.Client{
		Timeout: time.Second * DefaultTimeoutSeconds,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	},
}

func SetTransport(tr *http.Transport) {
	senderInstance.client.Transport = tr
}

type EndPoint struct {
	URL    string
	Secret string
}

func Send(endPoint EndPoint, data interface{}) error {
	return senderInstance.Send(endPoint, data)
}

func SendAll(endPoints []EndPoint, data interface{}) {
	for _, e := range endPoints {
		go func(e EndPoint) {
			senderInstance.Send(e, data)
		}(e)
	}
}

func SendTelegram(botToken, chatID, text string) error {
	return senderInstance.SendTelegram(botToken, chatID, text)
}

func SendHTTPAPI(method, targetURL string, headers map[string]string, data interface{}) error {
	return senderInstance.SendHTTPAPI(method, targetURL, headers, data)
}

func SendHTTPAPIRaw(method, targetURL string, headers map[string]string, body []byte) error {
	return senderInstance.SendHTTPAPIRaw(method, targetURL, headers, body)
}

func (ds defaultSender) SendTelegram(botToken, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	jsonData, err := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		log.Error(err)
		return err
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error(err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ds.client.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= MinHTTPStatusErrorCode {
		errMsg := fmt.Sprintf("telegram API returned a status code outside the 2XX range: %d", resp.StatusCode)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

func (ds defaultSender) Send(endPoint EndPoint, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Error(err)
		return err
	}

	req, err := http.NewRequest("POST", endPoint.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error(err)
		return err
	}
	signat, err := sign(endPoint.Secret, jsonData)
	if err != nil {
		log.Error(err)
		return err
	}
	req.Header.Set(SignatureHeader, fmt.Sprintf("%s=%s", Sha256Prefix, signat))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ds.client.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= MinHTTPStatusErrorCode {
		errMsg := fmt.Sprintf("http status of response: %s", resp.Status)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

func (ds defaultSender) SendHTTPAPI(method, targetURL string, headers map[string]string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Error(err)
		return err
	}
	return ds.SendHTTPAPIRaw(method, targetURL, headers, jsonData)
}

func (ds defaultSender) SendHTTPAPIRaw(method, targetURL string, headers map[string]string, body []byte) error {
	req, err := http.NewRequest(method, targetURL, bytes.NewBuffer(body))
	if err != nil {
		log.Error(err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ds.client.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= MinHTTPStatusErrorCode {
		errMsg := fmt.Sprintf("http status of response: %s", resp.Status)
		log.Error(errMsg)
		return errors.New(errMsg)
	}
	return nil
}

func sign(secret string, data []byte) (string, error) {
	hash1 := hmac.New(sha256.New, []byte(secret))
	_, err := hash1.Write(data)
	if err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(hash1.Sum(nil))
	return hexStr, nil
}
