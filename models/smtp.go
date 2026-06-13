package models

import (
	"crypto/tls"
	"errors"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gophish/gomail"
	"github.com/gophish/gophish/dialer"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/jinzhu/gorm"
)

// Dialer is a wrapper around a standard gomail.Dialer in order
// to implement the mailer.Dialer interface. This allows us to better
// separate the mailer package as opposed to forcing a connection
// between mailer and gomail.
type Dialer struct {
	*gomail.Dialer
}

// Dial wraps the gomail dialer's Dial command
func (d *Dialer) Dial() (mailer.Sender, error) {
	return d.Dialer.Dial()
}

// Interface types for a sending profile
const (
	InterfaceTypeSMTP          = "SMTP"
	InterfaceTypeHTTP          = "HTTP"
	InterfaceTypeGmail         = "Gmail"
	InterfaceTypeOutlookOAuth2 = "OutlookOAuth2"
)

// SMTP contains the attributes needed to handle the sending of campaign emails
type SMTP struct {
	Id               int64     `json:"id" gorm:"column:id; primary_key:yes"`
	UserId           int64     `json:"-" gorm:"column:user_id"`
	Interface        string    `json:"interface_type" gorm:"column:interface_type"`
	Name             string    `json:"name"`
	Host             string    `json:"host"`
	Username         string    `json:"username,omitempty"`
	Password         string    `json:"password,omitempty"`
	FromAddress      string    `json:"from_address"`
	IgnoreCertErrors bool      `json:"ignore_cert_errors"`
	Headers          []Header  `json:"headers"`
	ModifiedDate     time.Time `json:"modified_date"`

	// HTTP API interface fields. These are only used when Interface is
	// InterfaceTypeHTTP. They let a sending profile deliver email by calling
	// an arbitrary HTTP REST API (e.g. a mail-sending provider) rather than
	// connecting to an SMTP server.
	HTTPMethod      string `json:"http_method" gorm:"column:http_method"`
	HTTPURL         string `json:"http_url" gorm:"column:http_url"`
	HTTPHeaders     string `json:"http_headers" gorm:"column:http_headers"`
	HTTPContentType string `json:"http_content_type" gorm:"column:http_content_type"`
	HTTPBody        string `json:"http_body" gorm:"column:http_body"`

	// HTTP rate limiting. A value of 0 for a given window means that window is
	// not rate limited. Limits are enforced per sending profile and counted in
	// HTTP requests (so one batched request to many recipients counts once).
	HTTPRatePerSecond int `json:"http_rate_per_second" gorm:"column:http_rate_per_second"`
	HTTPRatePerMinute int `json:"http_rate_per_minute" gorm:"column:http_rate_per_minute"`
	HTTPRatePerHour   int `json:"http_rate_per_hour" gorm:"column:http_rate_per_hour"`

	// HTTPBatchSize is the number of recipients to include in a single HTTP
	// request. A value <= 1 sends one request per recipient.
	HTTPBatchSize int `json:"http_batch_size" gorm:"column:http_batch_size"`

	OutlookClientID        string `json:"outlook_client_id" gorm:"column:outlook_client_id"`
	OutlookTokenCache      string `json:"-" gorm:"column:outlook_token_cache"`
	OutlookAuthenticated   bool   `json:"outlook_authenticated" gorm:"-"`
	OutlookTokenCacheInput string `json:"outlook_token_cache_input" gorm:"-"`
}

// Header contains the fields and methods for a sending profile to have
// custom headers
type Header struct {
	Id     int64  `json:"-"`
	SMTPId int64  `json:"-"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// ErrFromAddressNotSpecified is thrown when there is no "From" address
// specified in the SMTP configuration
var ErrFromAddressNotSpecified = errors.New("No From Address specified")

// ErrInvalidFromAddress is thrown when the SMTP From field in the sending
// profiles containes a value that is not an email address
var ErrInvalidFromAddress = errors.New("Invalid SMTP From address because it is not an email address")

// ErrHostNotSpecified is thrown when there is no Host specified
// in the SMTP configuration
var ErrHostNotSpecified = errors.New("No SMTP Host specified")

// ErrInvalidHost indicates that the SMTP server string is invalid
var ErrInvalidHost = errors.New("Invalid SMTP server address")

// ErrHTTPURLNotSpecified is thrown when an HTTP sending profile has no URL
var ErrHTTPURLNotSpecified = errors.New("No HTTP URL specified")

// ErrHTTPMethodNotSpecified is thrown when an HTTP sending profile has no method
var ErrHTTPMethodNotSpecified = errors.New("No HTTP method specified")

// ErrHTTPBodyNotSpecified is thrown when an HTTP sending profile has no body
var ErrHTTPBodyNotSpecified = errors.New("No HTTP request body specified")

// ErrInvalidHTTPURL indicates that the HTTP URL is not a valid absolute URL
var ErrInvalidHTTPURL = errors.New("Invalid HTTP URL")

// ErrGmailAppPasswordNotSpecified is thrown when a Gmail profile has no app password
var ErrGmailAppPasswordNotSpecified = errors.New("No Gmail App Password specified")

// ErrOutlookClientIDNotSpecified is thrown when an Outlook OAuth2 profile has no Azure Client ID
var ErrOutlookClientIDNotSpecified = errors.New("No Azure Client ID specified")

// ErrOutlookNotAuthenticated is thrown when sending is attempted before completing OAuth2 authentication
var ErrOutlookNotAuthenticated = errors.New("Outlook OAuth2 profile not authenticated - open the profile settings, run the outlook_oauth2.py script, paste the token_cache.json content, and save")

// TableName specifies the database tablename for Gorm to use
func (s SMTP) TableName() string {
	return "smtp"
}

func (s *SMTP) AfterFind() error {
	s.OutlookAuthenticated = s.OutlookTokenCache != ""
	return nil
}

// Validate ensures that SMTP configs/connections are valid
func (s *SMTP) Validate() error {
	switch s.Interface {
	case InterfaceTypeHTTP:
		return s.validateHTTP()
	case InterfaceTypeGmail:
		return s.validateGmail()
	case InterfaceTypeOutlookOAuth2:
		return s.validateOutlookOAuth2()
	}
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case s.Host == "":
		return ErrHostNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	}
	_, err := mail.ParseAddress(s.FromAddress)
	if err != nil {
		return err
	}
	// Make sure addr is in host:port format
	hp := strings.Split(s.Host, ":")
	if len(hp) > 2 {
		return ErrInvalidHost
	} else if len(hp) < 2 {
		hp = append(hp, "25")
	}
	_, err = strconv.Atoi(hp[1])
	if err != nil {
		return ErrInvalidHost
	}
	return err
}

func (s *SMTP) validateGmail() error {
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	case s.Password == "":
		return ErrGmailAppPasswordNotSpecified
	}
	_, err := mail.ParseAddress(s.FromAddress)
	return err
}

func (s *SMTP) validateOutlookOAuth2() error {
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	case s.OutlookClientID == "":
		return ErrOutlookClientIDNotSpecified
	case s.OutlookTokenCache == "":
		return ErrOutlookNotAuthenticated
	}
	_, err := mail.ParseAddress(s.FromAddress)
	return err
}

// validateFromAddress validates
func validateFromAddress(email string) bool {
	r, _ := regexp.Compile("^([a-zA-Z0-9_\\-\\.]+)@([a-zA-Z0-9_\\-\\.]+)\\.([a-zA-Z]{2,18})$")
	return r.MatchString(email)
}

// validateHTTP ensures that an HTTP sending profile has the required fields
// and that the configured URL is a valid absolute HTTP(S) URL.
func (s *SMTP) validateHTTP() error {
	switch {
	case s.FromAddress == "":
		return ErrFromAddressNotSpecified
	case !validateFromAddress(s.FromAddress):
		return ErrInvalidFromAddress
	case s.HTTPURL == "":
		return ErrHTTPURLNotSpecified
	case s.HTTPMethod == "":
		return ErrHTTPMethodNotSpecified
	case s.HTTPBody == "":
		return ErrHTTPBodyNotSpecified
	}
	u, err := url.Parse(s.HTTPURL)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalidHTTPURL
	}
	return nil
}

// GetDialer returns a dialer for the given SMTP profile
func (s *SMTP) GetDialer() (mailer.Dialer, error) {
	switch s.Interface {
	case InterfaceTypeHTTP:
		// HTTP sending profiles deliver mail over an HTTP API instead of SMTP.
		return &HTTPDialer{profile: *s}, nil
	case InterfaceTypeGmail:
		// Gmail App Password: SMTP SSL on port 465, username = From address.
		d := gomail.NewWithDialer(dialer.Dialer(), "smtp.gmail.com", 465, s.FromAddress, s.Password)
		d.TLSConfig = &tls.Config{ServerName: "smtp.gmail.com"}
		hostname, err := os.Hostname()
		if err != nil {
			log.Error(err)
			hostname = "localhost"
		}
		d.LocalName = hostname
		return &Dialer{d}, nil
	case InterfaceTypeOutlookOAuth2:
		// Outlook OAuth2 requires authentication before sending.
		if s.OutlookTokenCache == "" {
			return nil, ErrOutlookNotAuthenticated
		}
		return &OutlookOAuth2Dialer{profile: *s}, nil
	}
	// Default: plain SMTP
	hp := strings.Split(s.Host, ":")
	if len(hp) < 2 {
		hp = append(hp, "25")
	}
	host := hp[0]
	port, err := strconv.Atoi(hp[1])
	if err != nil {
		log.Error(err)
		return nil, err
	}
	d := gomail.NewWithDialer(dialer.Dialer(), host, port, s.Username, s.Password)
	d.TLSConfig = &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: s.IgnoreCertErrors,
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Error(err)
		hostname = "localhost"
	}
	d.LocalName = hostname
	return &Dialer{d}, err
}

// GetSMTPs returns the SMTPs owned by the given user.
func GetSMTPs(uid int64) ([]SMTP, error) {
	ss := []SMTP{}
	err := db.Where("user_id=?", uid).Find(&ss).Error
	if err != nil {
		log.Error(err)
		return ss, err
	}
	for i := range ss {
		err = db.Where("smtp_id=?", ss[i].Id).Find(&ss[i].Headers).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			log.Error(err)
			return ss, err
		}
	}
	return ss, nil
}

// GetSMTP returns the SMTP, if it exists, specified by the given id and user_id.
func GetSMTP(id int64, uid int64) (SMTP, error) {
	s := SMTP{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	err = db.Where("smtp_id=?", s.Id).Find(&s.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
		return s, err
	}
	return s, err
}

// GetSMTPByName returns the SMTP, if it exists, specified by the given name and user_id.
func GetSMTPByName(n string, uid int64) (SMTP, error) {
	s := SMTP{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&s).Error
	if err != nil {
		log.Error(err)
		return s, err
	}
	err = db.Where("smtp_id=?", s.Id).Find(&s.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
	}
	return s, err
}

// PostSMTP creates a new SMTP in the database.
func PostSMTP(s *SMTP) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	// Insert into the DB
	err = db.Save(s).Error
	if err != nil {
		log.Error(err)
	}
	// Save custom headers
	for i := range s.Headers {
		s.Headers[i].SMTPId = s.Id
		err := db.Save(&s.Headers[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return err
}

// PutSMTP edits an existing SMTP in the database.
// Per the PUT Method RFC, it presumes all data for a SMTP is provided.
func PutSMTP(s *SMTP) error {
	err := s.Validate()
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("id=?", s.Id).Save(s).Error
	if err != nil {
		log.Error(err)
	}
	// Delete all custom headers, and replace with new ones
	err = db.Where("smtp_id=?", s.Id).Delete(&Header{}).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
		return err
	}
	// Save custom headers
	for i := range s.Headers {
		s.Headers[i].SMTPId = s.Id
		err := db.Save(&s.Headers[i]).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return err
}

// DeleteSMTP deletes an existing SMTP in the database.
// An error is returned if a SMTP with the given user id and SMTP id is not found.
func DeleteSMTP(id int64, uid int64) error {
	// Delete all custom headers
	err := db.Where("smtp_id=?", id).Delete(&Header{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("user_id=?", uid).Delete(SMTP{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}
