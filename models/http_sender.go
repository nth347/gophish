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
	"text/template"
	"time"

	"github.com/gophish/gophish/dialer"
	"github.com/gophish/gophish/mailer"
)

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

var httpSendTimeout = 30 * time.Second

type HTTPDialer struct {
	profile SMTP
}

func (d *HTTPDialer) Dial() (mailer.Sender, error) {
	return nil, fmt.Errorf("HTTP sending profiles do not support SMTP dialing")
}

func (d *HTTPDialer) BatchSize() int {
	if d.profile.HTTPBatchSize > 1 {
		return d.profile.HTTPBatchSize
	}
	return 1
}

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

func (d *HTTPDialer) SendEmail(ctx context.Context, content *mailer.EmailContent) error {
	p := d.profile

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

