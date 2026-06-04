package models

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gophish/gophish/mailer"
)

// TestHTTPSenderSendEmail verifies that an HTTP sending profile renders its
// body template with the email content and issues a request with the
// configured method, headers, and content type. This test does not require a
// database connection.
func TestHTTPSenderSendEmail(t *testing.T) {
	var (
		gotMethod      string
		gotAuth        string
		gotContentType string
		gotBody        map[string]string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	profile := SMTP{
		Interface:       InterfaceTypeHTTP,
		FromAddress:     "sender@example.com",
		HTTPMethod:      "POST",
		HTTPURL:         srv.URL,
		HTTPHeaders:     "Authorization: Bearer test-token\nAccept: application/json",
		HTTPContentType: "application/json",
		HTTPBody: `{
			"to": {{.To | json}},
			"from": {{.From | json}},
			"subject": {{.Subject | json}},
			"html": {{.HTML | json}},
			"text": {{.Text | json}}
		}`,
	}

	dialer := &HTTPDialer{profile: profile}
	content := &mailer.EmailContent{
		From:    "sender@example.com",
		To:      "victim@example.com",
		Subject: "Please change your passwords",
		HTML:    "<h1>Welcome!</h1>",
		Text:    "Welcome!",
	}

	if err := dialer.SendEmail(context.Background(), content); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected method POST, got %q", gotMethod)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization header to be passed through, got %q", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
	if gotBody["to"] != "victim@example.com" {
		t.Errorf("expected recipient placeholder rendered, got %q", gotBody["to"])
	}
	if gotBody["from"] != "sender@example.com" {
		t.Errorf("expected from placeholder rendered, got %q", gotBody["from"])
	}
	if gotBody["subject"] != "Please change your passwords" {
		t.Errorf("expected subject placeholder rendered, got %q", gotBody["subject"])
	}
	if gotBody["html"] != "<h1>Welcome!</h1>" {
		t.Errorf("expected html placeholder rendered, got %q", gotBody["html"])
	}
}

// TestHTTPSenderNon2xx verifies that a non-2xx response is reported as an error.
func TestHTTPSenderNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer srv.Close()

	dialer := &HTTPDialer{profile: SMTP{
		Interface:  InterfaceTypeHTTP,
		HTTPMethod: "POST",
		HTTPURL:    srv.URL,
		HTTPBody:   `{"to": {{.To | json}}}`,
	}}

	err := dialer.SendEmail(context.Background(), &mailer.EmailContent{To: "victim@example.com"})
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}
}

// TestHTTPSenderMultipleRecipients verifies that a request can carry multiple
// recipients as a JSON array via the {{.Recipients}} placeholder.
func TestHTTPSenderMultipleRecipients(t *testing.T) {
	var gotTo []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			To []string `json:"to"`
		}
		raw, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(raw, &body)
		gotTo = body.To
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dialer := &HTTPDialer{profile: SMTP{
		Interface:  InterfaceTypeHTTP,
		HTTPMethod: "POST",
		HTTPURL:    srv.URL,
		HTTPBody:   `{"to": {{.Recipients | json}}}`,
	}}

	content := &mailer.EmailContent{
		To:         "user1@example.com",
		Recipients: []string{"user1@example.com", "user2@example.com"},
	}
	if err := dialer.SendEmail(context.Background(), content); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}
	if len(gotTo) != 2 || gotTo[0] != "user1@example.com" || gotTo[1] != "user2@example.com" {
		t.Errorf("expected both recipients in the request, got %v", gotTo)
	}
}

// TestHTTPTestEmail verifies that the "Send Test Email" path works for an HTTP
// sending profile: an EmailRequest renders its content via GenerateHTTP and is
// delivered through the HTTPDialer, exactly as the mailer's sendMailHTTP path
// does for a single test message.
func TestHTTPTestEmail(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	profile := SMTP{
		Interface:       InterfaceTypeHTTP,
		FromAddress:     "sender@example.com",
		HTTPMethod:      "POST",
		HTTPURL:         srv.URL,
		HTTPContentType: "application/json",
		HTTPBody: `{
			"to": "{{.To}}",
			"subject": {{.Subject | json}},
			"text": {{.Text | json}}
		}`,
	}

	req := &EmailRequest{
		Template:    Template{Subject: "Default Email from Gophish", Text: "It works! {{.From}}"},
		SMTP:        profile,
		FromAddress: "sender@example.com",
		RId:         "preview-test",
		BaseRecipient: BaseRecipient{
			Email:     "tester@example.com",
			FirstName: "Test",
			LastName:  "User",
		},
	}

	// This mirrors what mailer.sendMailHTTP does for a single test message.
	content, err := req.GenerateHTTP()
	if err != nil {
		t.Fatalf("GenerateHTTP returned error: %v", err)
	}
	dialer := &HTTPDialer{profile: profile}
	if err := dialer.SendEmail(context.Background(), content); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	if gotBody["to"] != "tester@example.com" {
		t.Errorf("expected recipient tester@example.com, got %v", gotBody["to"])
	}
	if gotBody["subject"] != "Default Email from Gophish" {
		t.Errorf("expected default subject, got %v", gotBody["subject"])
	}
	if gotBody["text"] != "It works! sender@example.com" {
		t.Errorf("expected rendered text body, got %v", gotBody["text"])
	}
}

// TestHTTPSenderAttachments verifies that attachments are rendered into the
// request body as a JSON array via the {{.Attachments}} placeholder, matching
// the schema used by providers such as Cloudflare.
func TestHTTPSenderAttachments(t *testing.T) {
	var body struct {
		Attachments []struct {
			Content     string `json:"content"`
			Filename    string `json:"filename"`
			Type        string `json:"type"`
			Disposition string `json:"disposition"`
		} `json:"attachments"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("server received invalid JSON: %v\nbody: %s", err, raw)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dialer := &HTTPDialer{profile: SMTP{
		Interface:  InterfaceTypeHTTP,
		HTTPMethod: "POST",
		HTTPURL:    srv.URL,
		HTTPBody: `{
			"to": "{{.To}}",
			"attachments": [{{range $i, $a := .Attachments}}{{if $i}},{{end}}{
				"content": {{$a.Content | json}},
				"filename": {{$a.Filename | json}},
				"type": {{$a.Type | json}},
				"disposition": "attachment"
			}{{end}}]
		}`,
	}}

	content := &mailer.EmailContent{
		To: "customer@example.com",
		Attachments: []mailer.EmailAttachment{
			{Content: "JVBERi0xLjQK", Filename: "invoice-12345.pdf", Type: "application/pdf"},
			{Content: "aGVsbG8=", Filename: "note.txt", Type: "text/plain"},
		},
	}
	if err := dialer.SendEmail(context.Background(), content); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	if len(body.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(body.Attachments))
	}
	if body.Attachments[0].Content != "JVBERi0xLjQK" || body.Attachments[0].Filename != "invoice-12345.pdf" || body.Attachments[0].Type != "application/pdf" {
		t.Errorf("unexpected first attachment: %+v", body.Attachments[0])
	}
	if body.Attachments[1].Filename != "note.txt" {
		t.Errorf("unexpected second attachment: %+v", body.Attachments[1])
	}
}

// TestBuildHTTPAttachments verifies that template attachments are base64
// re-encoded after their template variables are applied.
func TestBuildHTTPAttachments(t *testing.T) {
	ptx := PhishingTemplateContext{
		BaseRecipient: BaseRecipient{Email: "victim@example.com", FirstName: "Vic"},
		From:          "sender@example.com",
	}
	// "Hello Vic" once {{.FirstName}} is applied, then base64 encoded.
	attachments := []Attachment{
		{Name: "greeting.txt", Type: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("Hello {{.FirstName}}"))},
	}
	got, err := buildHTTPAttachments(attachments, ptx)
	if err != nil {
		t.Fatalf("buildHTTPAttachments returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
	decodedBytes, _ := base64.StdEncoding.DecodeString(got[0].Content)
	decoded := string(decodedBytes)
	if decoded != "Hello Vic" {
		t.Errorf("expected template-applied content 'Hello Vic', got %q", decoded)
	}
	if got[0].Filename != "greeting.txt" || got[0].Type != "text/plain" {
		t.Errorf("unexpected attachment metadata: %+v", got[0])
	}
}

// TestHTTPRateLimiter verifies that the per-second limit throttles requests.
func TestHTTPRateLimiter(t *testing.T) {
	l := &httpRateLimiter{perSecond: 2}
	ctx := context.Background()

	start := time.Now()
	// The first two should be immediate; the third must wait for the 1s window.
	for i := 0; i < 3; i++ {
		if err := l.Wait(ctx); err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
	}
	if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
		t.Errorf("expected the third request to be throttled by ~1s, only waited %v", elapsed)
	}
}

// TestHTTPRateLimiterCancel verifies that a cancelled context unblocks Wait.
func TestHTTPRateLimiterCancel(t *testing.T) {
	l := &httpRateLimiter{perHour: 1}
	ctx, cancel := context.WithCancel(context.Background())

	// Consume the single allowed request.
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("first Wait returned error: %v", err)
	}

	var done int32
	go func() {
		// The second request would block for an hour; cancellation must
		// unblock it promptly.
		l.Wait(ctx)
		atomic.StoreInt32(&done, 1)
	}()
	cancel()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&done) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Wait did not return after context cancellation")
}

// TestSMTPValidateHTTP verifies the validation rules for HTTP sending profiles.
func TestSMTPValidateHTTP(t *testing.T) {
	valid := SMTP{
		Interface:   InterfaceTypeHTTP,
		FromAddress: "sender@example.com",
		HTTPMethod:  "POST",
		HTTPURL:     "https://api.example.com/send",
		HTTPBody:    `{"to": "{{.To}}"}`,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid HTTP profile, got error: %v", err)
	}

	cases := map[string]SMTP{
		"missing url":    {Interface: InterfaceTypeHTTP, FromAddress: "s@e.com", HTTPMethod: "POST", HTTPBody: "x"},
		"missing method": {Interface: InterfaceTypeHTTP, FromAddress: "s@e.com", HTTPURL: "https://e.com", HTTPBody: "x"},
		"missing body":   {Interface: InterfaceTypeHTTP, FromAddress: "s@e.com", HTTPMethod: "POST", HTTPURL: "https://e.com"},
		"bad url":        {Interface: InterfaceTypeHTTP, FromAddress: "s@e.com", HTTPMethod: "POST", HTTPURL: "not-a-url", HTTPBody: "x"},
	}
	for name, c := range cases {
		c := c
		if err := c.Validate(); err == nil {
			t.Errorf("case %q: expected validation error, got nil", name)
		}
	}
}
