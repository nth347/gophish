package models

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/smtp"
	"path/filepath"
	"strings"
	"time"

	"github.com/gophish/gophish/mailer"
)

const (
	gmailSMTPHost   = "smtp.gmail.com"
	gmailSMTPPort   = 465
	gmailMailDomain = "mail.gmail.com"
)

// GmailSMTPSender builds a raw MIME message with Gmail's exact structure
// (multipart hierarchy, base64 encoding, header order, boundary format) and
// delivers it via an implicit-TLS SMTP connection to smtp.gmail.com:465.
//
// It implements mailer.Dialer and mailer.HTTPSender so the mail worker routes
// messages through SendEmail instead of the standard SMTP path, giving us
// full control over the serialised message bytes.
type GmailSMTPSender struct {
	profile SMTP
}

// Dial satisfies mailer.Dialer; actual delivery goes through SendEmail.
func (g *GmailSMTPSender) Dial() (mailer.Sender, error) {
	return nil, fmt.Errorf("GmailSMTPSender uses SendEmail, not Dial")
}

// BatchSize sends one SMTP DATA transaction per recipient.
func (g *GmailSMTPSender) BatchSize() int { return 1 }

// SendEmail builds the raw RFC 2822 message and delivers it to smtp.gmail.com:465.
func (g *GmailSMTPSender) SendEmail(ctx context.Context, content *mailer.EmailContent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	raw, err := buildGmailMessage(content)
	if err != nil {
		return fmt.Errorf("Gmail: build message: %v", err)
	}
	return gmailDialAndSend(g.profile.FromAddress, g.profile.Password, content.Recipients, raw)
}

func gmailDialAndSend(from, password string, recipients []string, raw []byte) error {
	tlsCfg := &tls.Config{ServerName: gmailSMTPHost}
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", gmailSMTPHost, gmailSMTPPort), tlsCfg)
	if err != nil {
		return fmt.Errorf("Gmail: TLS dial: %v", err)
	}

	// smtp.NewClient detects *tls.Conn and sets c.tls = true, which allows
	// smtp.PlainAuth to proceed without an "unencrypted connection" error.
	client, err := smtp.NewClient(conn, gmailSMTPHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("Gmail: SMTP client: %v", err)
	}
	defer client.Quit()

	if err := client.Auth(smtp.PlainAuth("", from, password, gmailSMTPHost)); err != nil {
		return fmt.Errorf("Gmail: auth: %v", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("Gmail: MAIL FROM: %v", err)
	}
	for _, r := range recipients {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("Gmail: RCPT TO %s: %v", r, err)
		}
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("Gmail: DATA: %v", err)
	}
	if _, err := wc.Write(raw); err != nil {
		wc.Close()
		return err
	}
	return wc.Close()
}

// buildGmailMessage constructs raw RFC 2822 bytes that match Gmail webmail's
// MIME structure, header order, boundary format and base64 encoding.
//
// Structure mirrors the sample-gmail.eml reference:
//
//	multipart/mixed  (when attachments present)
//	  multipart/alternative  (when both text and html)
//	    text/plain; charset="utf-8"  base64
//	    text/html;  charset="utf-8"  base64
//	  <attachments>
//
//	multipart/alternative  (text + html, no attachments)
//	  text/plain; charset="utf-8"  base64
//	  text/html;  charset="utf-8"  base64
//
//	text/html or text/plain  (single body part)
func buildGmailMessage(content *mailer.EmailContent) ([]byte, error) {
	outerBoundary, err := gmailBoundary()
	if err != nil {
		return nil, err
	}
	innerBoundary, err := gmailBoundary()
	if err != nil {
		return nil, err
	}
	msgID, err := gmailMessageID()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	hasText := content.Text != ""
	hasHTML := content.HTML != ""
	hasAtts := len(content.Attachments) > 0
	hasBoth := hasText && hasHTML

	// Header order matches Gmail webmail:
	// From → Date → Message-ID → Subject → To → Content-Type → MIME-Version
	gmailHdr(&buf, "From", gmailFromHeader(content.FromName, content.From))
	gmailHdr(&buf, "Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	gmailHdr(&buf, "Message-ID", msgID)
	gmailHdr(&buf, "Subject", gmailEncodeWord(content.Subject))
	gmailHdr(&buf, "To", content.To)

	switch {
	case hasAtts:
		// multipart/mixed wraps (multipart/alternative or single body) + attachments.
		gmailHdr(&buf, "Content-Type", `multipart/mixed; boundary="`+outerBoundary+`"`)
		gmailHdr(&buf, "MIME-Version", "1.0")
		buf.WriteString("\r\n")

		// First outer part: the body (alternative or single).
		buf.WriteString("--" + outerBoundary + "\r\n")
		if hasBoth {
			gmailHdr(&buf, "Content-Type", `multipart/alternative; boundary="`+innerBoundary+`"`)
			buf.WriteString("\r\n")
			gmailBodyPart(&buf, innerBoundary, "text/plain", content.Text)
			gmailBodyPart(&buf, innerBoundary, "text/html", content.HTML)
			buf.WriteString("--" + innerBoundary + "--\r\n")
		} else if hasHTML {
			gmailInlineBody(&buf, "text/html", content.HTML)
		} else {
			gmailInlineBody(&buf, "text/plain", content.Text)
		}
		buf.WriteString("\r\n") // CRLF before next outer boundary

		// Attachment parts.
		for _, att := range content.Attachments {
			buf.WriteString("--" + outerBoundary + "\r\n")
			if gmailIsInline(att.Filename) {
				gmailHdr(&buf, "Content-Type", att.Type+`; name="`+att.Filename+`"`)
				gmailHdr(&buf, "Content-Disposition", `inline; filename="`+att.Filename+`"`)
				gmailHdr(&buf, "Content-ID", "<"+att.Filename+">")
			} else {
				gmailHdr(&buf, "Content-Type", att.Type+`; name="`+att.Filename+`"`)
				gmailHdr(&buf, "Content-Disposition", `attachment; filename="`+att.Filename+`"`)
			}
			gmailHdr(&buf, "Content-Transfer-Encoding", "base64")
			buf.WriteString("\r\n")
			gmailWrapBase64(&buf, att.Content)
			buf.WriteString("\r\n") // CRLF before next boundary
		}
		buf.WriteString("--" + outerBoundary + "--\r\n")

	case hasBoth:
		// multipart/alternative, no attachments.
		gmailHdr(&buf, "Content-Type", `multipart/alternative; boundary="`+innerBoundary+`"`)
		gmailHdr(&buf, "MIME-Version", "1.0")
		buf.WriteString("\r\n")
		gmailBodyPart(&buf, innerBoundary, "text/plain", content.Text)
		gmailBodyPart(&buf, innerBoundary, "text/html", content.HTML)
		buf.WriteString("--" + innerBoundary + "--\r\n")

	default:
		// Single body part (HTML-only or text-only).
		ct, body := "text/html", content.HTML
		if !hasHTML {
			ct, body = "text/plain", content.Text
		}
		gmailHdr(&buf, "Content-Type", ct+`; charset="utf-8"`)
		gmailHdr(&buf, "MIME-Version", "1.0")
		gmailHdr(&buf, "Content-Transfer-Encoding", "base64")
		buf.WriteString("\r\n")
		gmailB64Lines(&buf, []byte(body))
	}

	return buf.Bytes(), nil
}

// gmailHdr writes one RFC 2822 header line.
func gmailHdr(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key + ": " + value + "\r\n")
}

// gmailBodyPart writes a MIME body part inside a multipart boundary.
// The trailing \r\n acts as the required delimiter-CRLF for the next boundary.
func gmailBodyPart(buf *bytes.Buffer, boundary, ct, body string) {
	buf.WriteString("--" + boundary + "\r\n")
	gmailHdr(buf, "Content-Type", ct+`; charset="utf-8"`)
	gmailHdr(buf, "Content-Transfer-Encoding", "base64")
	buf.WriteString("\r\n")
	gmailB64Lines(buf, []byte(body))
	buf.WriteString("\r\n")
}

// gmailInlineBody writes a body part inline in the outer multipart/mixed
// (used when only one content type is present alongside attachments).
func gmailInlineBody(buf *bytes.Buffer, ct, body string) {
	gmailHdr(buf, "Content-Type", ct+`; charset="utf-8"`)
	gmailHdr(buf, "Content-Transfer-Encoding", "base64")
	buf.WriteString("\r\n")
	gmailB64Lines(buf, []byte(body))
}

// gmailB64Lines base64-encodes data with 76-character line wrapping per RFC 2045 §6.8.
func gmailB64Lines(buf *bytes.Buffer, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		buf.WriteString(enc[:76])
		buf.WriteString("\r\n")
		enc = enc[76:]
	}
	if len(enc) > 0 {
		buf.WriteString(enc)
		buf.WriteString("\r\n")
	}
}

// gmailWrapBase64 re-wraps an already-base64-encoded string to 76-char lines.
// att.Content from buildHTTPAttachments is a single-line base64.StdEncoding string.
func gmailWrapBase64(buf *bytes.Buffer, b64 string) {
	for len(b64) > 76 {
		buf.WriteString(b64[:76])
		buf.WriteString("\r\n")
		b64 = b64[76:]
	}
	if len(b64) > 0 {
		buf.WriteString(b64)
		buf.WriteString("\r\n")
	}
}

// gmailIsInline returns true for image types that use inline (Content-ID) disposition.
func gmailIsInline(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif":
		return true
	}
	return false
}

// gmailFromHeader formats the RFC 2822 From header value with an optional
// display name. Non-ASCII names are encoded as =?UTF-8?B?...?=.
func gmailFromHeader(name, addr string) string {
	if name == "" {
		return addr
	}
	return gmailEncodeWord(name) + " <" + addr + ">"
}

// gmailEncodeWord encodes s as =?UTF-8?B?...?= (uppercase B, matching Gmail)
// when it contains non-ASCII characters; otherwise returns s unchanged.
// Long strings are split into multiple encoded words by Go's mime package.
func gmailEncodeWord(s string) string {
	for _, r := range s {
		if r > 127 {
			// Go's mime.BEncoding uses lowercase 'b'; Gmail uses uppercase 'B'.
			// Both are valid per RFC 2047; we normalise to uppercase to match.
			encoded := mime.BEncoding.Encode("UTF-8", s)
			return strings.ReplaceAll(encoded, "=?UTF-8?b?", "=?UTF-8?B?")
		}
	}
	return s
}

// gmailBoundary generates a 28-character boundary matching Gmail's format:
// 12 zero hex chars + 16 random hex chars (e.g. "00000000000020e430065437841a").
func gmailBoundary() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "000000000000" + hex.EncodeToString(b), nil
}

// gmailMessageID generates a Message-ID that matches Gmail's format:
// <[base64-random]@mail.gmail.com>
func gmailMessageID() (string, error) {
	b := make([]byte, 26)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	enc := strings.TrimRight(base64.StdEncoding.EncodeToString(b), "=")
	return "<" + enc + "@" + gmailMailDomain + ">", nil
}
