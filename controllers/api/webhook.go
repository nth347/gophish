package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/webhook"
	"github.com/gorilla/mux"
)

// Webhooks returns a list of webhooks, both active and disabled
func (as *Server) Webhooks(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		whs, err := models.GetWebhooks()
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, whs, http.StatusOK)

	case r.Method == "POST":
		wh := models.Webhook{}
		err := json.NewDecoder(r.Body).Decode(&wh)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostWebhook(&wh)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, wh, http.StatusCreated)
	}
}

// Webhook returns details of a single webhook specified by "id" parameter
func (as *Server) Webhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	wh, err := models.GetWebhook(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Webhook not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, wh, http.StatusOK)

	case r.Method == "DELETE":
		err = models.DeleteWebhook(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		log.Infof("Deleted webhook with id: %d", id)
		JSONResponse(w, models.Response{Success: true, Message: "Webhook deleted Successfully!"}, http.StatusOK)

	case r.Method == "PUT":
		wh = models.Webhook{}
		err = json.NewDecoder(r.Body).Decode(&wh)
		if err != nil {
			log.Errorf("error decoding webhook: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		wh.Id = id
		err = models.PutWebhook(&wh)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, wh, http.StatusOK)
	}
}

// TestWebhookRequest sends a test HTTP API request using the webhook config
// supplied in the request body, without requiring the webhook to be saved
// first. Only the http_api type is accepted; use the existing validate endpoint
// for standard/telegram webhooks.
func (as *Server) TestWebhookRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	wh := models.Webhook{}
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON"}, http.StatusBadRequest)
		return
	}
	if wh.URL == "" {
		JSONResponse(w, models.Response{Success: false, Message: "URL is required"}, http.StatusBadRequest)
		return
	}
	method := wh.APIMethod
	if method == "" {
		method = "POST"
	}
	var headers map[string]string
	if wh.APIHeaders != "" {
		if err := json.Unmarshal([]byte(wh.APIHeaders), &headers); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid api_headers JSON"}, http.StatusBadRequest)
			return
		}
	}
	testEvent := &models.Event{
		CampaignId: 0,
		Email:      "test@example.com",
		Time:       time.Now().UTC(),
		Message:    models.EventDataSubmit,
		Details:    `{"payload":{"username":["test_user"],"password":["test_password"]},"browser":{"address":"127.0.0.1","user-agent":"Gophish Test"}}`,
	}
	if err := webhook.SendHTTPAPI(method, wh.URL, headers, testEvent); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Test request sent successfully"}, http.StatusOK)
}

// ValidateWebhook makes an HTTP request to a specified remote url to ensure that it's valid.
func (as *Server) ValidateWebhook(w http.ResponseWriter, r *http.Request) {
	type validationEvent struct {
		Success bool `json:"success"`
	}
	switch {
	case r.Method == "POST":
		vars := mux.Vars(r)
		id, _ := strconv.ParseInt(vars["id"], 0, 64)
		wh, err := models.GetWebhook(id)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		if wh.Type == models.WebhookTypeTelegram {
			err = webhook.SendTelegram(wh.TelegramBotToken, wh.TelegramChatID, "🎣 Gophish: webhook test successful.")
		} else if wh.Type == models.WebhookTypeHTTPAPI {
			method := wh.APIMethod
			if method == "" {
				method = "POST"
			}
			payload := validationEvent{Success: true}
			var headers map[string]string
			if wh.APIHeaders != "" {
				if jsonErr := json.Unmarshal([]byte(wh.APIHeaders), &headers); jsonErr != nil {
					headers = nil
				}
			}
			err = webhook.SendHTTPAPI(method, wh.URL, headers, payload)
		} else {
			payload := validationEvent{Success: true}
			err = webhook.Send(webhook.EndPoint{URL: wh.URL, Secret: wh.Secret}, payload)
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, wh, http.StatusOK)
	}
}
