package models

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gophish/gophish/dialer"
	"github.com/gophish/gophish/mailer"
)

// buildHTTPAttachments renders each template attachment (applying any phishing
// template variables) and base64-encodes the result so it can be embedded into
// an HTTP API request body via the {{.Attachments}} placeholder.
func buildHTTPAttachments(attachments []Attachment, ptx PhishingTemplateContext) ([]mailer.EmailAttachment, error) {
	result := make([]mailer.EmailAttachment, 0, len(attachments))
	for _, a := range attachments {
		reader, err := a.ApplyTemplate(ptx)
		if err != nil {
			return nil, err
		}
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		result = append(result, mailer.EmailAttachment{
			Content:  base64.StdEncoding.EncodeToString(data),
			Filename: a.Name,
			Type:     a.Type,
		})
	}
	return result, nil
}

// httpSendTimeout is the maximum amount of time to wait for an HTTP API to
// respond to a send request.
var httpSendTimeout = 30 * time.Second

// HTTPDialer implements the mailer.Dialer and mailer.HTTPSender interfaces. It
// delivers email by issuing an HTTP request to an arbitrary REST API as
// configured on the sending profile. Because it implements mailer.HTTPSender,
// the mail worker routes messages through SendEmail rather than the SMTP flow.
type HTTPDialer struct {
	profile SMTP
}

// Dial satisfies the mailer.Dialer interface. The HTTP transport is detected
// via the mailer.HTTPSender interface before Dial is ever called, so this is
// only here to satisfy the interface and returns the dialer itself.
func (d *HTTPDialer) Dial() (mailer.Sender, error) {
	return nil, fmt.Errorf("HTTP sending profiles do not support SMTP dialing")
}

// BatchSize reports how many recipients should be combined into a single HTTP
// request, as configured on the sending profile. A value <= 1 sends one
// request per recipient.
func (d *HTTPDialer) BatchSize() int {
	if d.profile.HTTPBatchSize > 1 {
		return d.profile.HTTPBatchSize
	}
	return 1
}

// bodyTemplateFuncs returns the template functions made available to the HTTP
// request body template. The "json" function JSON-encodes a value (including
// surrounding quotes for strings), which is the safe way to embed rendered
// HTML/text into a JSON request body.
func bodyTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}
}

// SendEmail renders the configured request body template using the provided
// email content and issues the HTTP request described by the sending profile.
// It returns an error if the request fails or if the API responds with a
// non-2xx status code. Before issuing the request it blocks as needed to
// respect the profile's configured rate limits.
func (d *HTTPDialer) SendEmail(ctx context.Context, content *mailer.EmailContent) error {
	p := d.profile

	// Block until sending is permitted by the per-profile rate limiter.
	if err := getHTTPLimiter(p).Wait(ctx); err != nil {
		return err
	}

	// Render the body template with the email content. The body template can
	// reference {{.To}}, {{.From}}, {{.FromName}}, {{.Subject}}, {{.HTML}},
	// {{.Text}}, {{.Recipients}} and {{.Attachments}} (each attachment exposes
	// .Content (base64), .Filename and .Type), and may use the {{... | json}}
	// function to safely escape values.
	tmpl, err := template.New("http_body").Funcs(bodyTemplateFuncs()).Parse(p.HTTPBody)
	if err != nil {
		return fmt.Errorf("invalid HTTP body template: %v", err)
	}
	buff := &bytes.Buffer{}
	if err := tmpl.Execute(buff, content); err != nil {
		return fmt.Errorf("error rendering HTTP body template: %v", err)
	}

	method := strings.ToUpper(strings.TrimSpace(p.HTTPMethod))
	req, err := http.NewRequestWithContext(ctx, method, p.HTTPURL, bytes.NewReader(buff.Bytes()))
	if err != nil {
		return err
	}

	// Apply the configured headers (one "Key: Value" pair per line). This is
	// where an Authorization header carrying an API token is typically set.
	for _, line := range strings.Split(p.HTTPHeaders, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	// An explicit content type takes precedence over any Content-Type provided
	// in the headers field.
	if strings.TrimSpace(p.HTTPContentType) != "" {
		req.Header.Set("Content-Type", strings.TrimSpace(p.HTTPContentType))
	}

	transport := &http.Transport{
		DialContext: dialer.Dialer().DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: p.IgnoreCertErrors,
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   httpSendTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return fmt.Errorf("HTTP API returned status %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

// httpRateLimiter is a simple sliding-window rate limiter that enforces
// per-second, per-minute and per-hour limits simultaneously. A limit of 0 for
// a given window disables rate limiting for that window.
type httpRateLimiter struct {
	mu        sync.Mutex
	perSecond int
	perMinute int
	perHour   int
	sec       []time.Time
	min       []time.Time
	hour      []time.Time
}

// windowWait prunes timestamps that have aged out of the window and returns how
// long the caller must wait before another request is permitted within that
// window. A return of 0 means the request may proceed immediately.
func windowWait(times *[]time.Time, limit int, window time.Duration, now time.Time) time.Duration {
	if limit <= 0 {
		return 0
	}
	cutoff := now.Add(-window)
	pruned := (*times)[:0]
	for _, t := range *times {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	*times = pruned
	if len(pruned) < limit {
		return 0
	}
	// The window is full; wait until the oldest request ages out.
	return pruned[0].Add(window).Sub(now)
}

// Wait blocks until a request is permitted under all configured rate limits or
// the context is cancelled.
func (l *httpRateLimiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		wait := time.Duration(0)
		if d := windowWait(&l.sec, l.perSecond, time.Second, now); d > wait {
			wait = d
		}
		if d := windowWait(&l.min, l.perMinute, time.Minute, now); d > wait {
			wait = d
		}
		if d := windowWait(&l.hour, l.perHour, time.Hour, now); d > wait {
			wait = d
		}
		if wait <= 0 {
			// Permitted: record this request in every active window.
			if l.perSecond > 0 {
				l.sec = append(l.sec, now)
			}
			if l.perMinute > 0 {
				l.min = append(l.min, now)
			}
			if l.perHour > 0 {
				l.hour = append(l.hour, now)
			}
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// limiterEntry stores a rate limiter alongside the configuration it was created
// with, so the limiter can be rebuilt if the profile's limits change.
type limiterEntry struct {
	limiter   *httpRateLimiter
	perSecond int
	perMinute int
	perHour   int
}

var (
	httpLimiterMu       sync.Mutex
	httpLimiterRegistry = map[int64]*limiterEntry{}
)

// getHTTPLimiter returns the rate limiter for the given sending profile,
// creating one (or replacing an outdated one) as needed. Limiters are keyed by
// profile ID so the limits are enforced across all of a profile's send batches
// for the lifetime of the process.
func getHTTPLimiter(p SMTP) *httpRateLimiter {
	httpLimiterMu.Lock()
	defer httpLimiterMu.Unlock()
	if e, ok := httpLimiterRegistry[p.Id]; ok &&
		e.perSecond == p.HTTPRatePerSecond &&
		e.perMinute == p.HTTPRatePerMinute &&
		e.perHour == p.HTTPRatePerHour {
		return e.limiter
	}
	l := &httpRateLimiter{
		perSecond: p.HTTPRatePerSecond,
		perMinute: p.HTTPRatePerMinute,
		perHour:   p.HTTPRatePerHour,
	}
	httpLimiterRegistry[p.Id] = &limiterEntry{
		limiter:   l,
		perSecond: p.HTTPRatePerSecond,
		perMinute: p.HTTPRatePerMinute,
		perHour:   p.HTTPRatePerHour,
	}
	return l
}
