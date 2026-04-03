package apiserver

import (
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// ValidateEmail performs basic email validation and rejects injection attempts.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > 254 {
		return errors.New("email too long")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return errors.New("invalid email format")
	}
	// Reject header injection characters
	if strings.ContainsAny(email, "\r\n\x00") {
		return errors.New("email contains invalid characters")
	}
	return nil
}

// sanitizeHeader removes CR/LF from header values to prevent header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// Mailer sends transactional emails.
type Mailer interface {
	SendLicenseKey(to, productName, tier, licenseKey string) error
	SendCancellation(to, productName string) error
	// Send sends a plain-text email to one recipient.
	Send(to, subject, body string) error
}

// SMTPMailer sends emails via SMTP.
type SMTPMailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	FromName string
}

// SMTPMailerFromEnv creates a mailer from environment variables.
func SMTPMailerFromEnv() *SMTPMailer {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return nil // No SMTP = no email
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	return &SMTPMailer{
		Host:     host,
		Port:     port,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		FromName: "Stockyard",
	}
}

// SendLicenseKey sends the license key to the customer.
func (m *SMTPMailer) SendLicenseKey(to, productName, tier, licenseKey string) error {
	subject := fmt.Sprintf("Your %s license key", productName)

	// Determine the correct env var name for this product
	envVar := "STOCKYARD_LICENSE_KEY"
	slug := strings.ToLower(productName)
	slug = strings.ReplaceAll(slug, " ", "")
	if isKnownTool(slug) {
		// All tools use STOCKYARD_LICENSE_KEY (unified env var)
		_ = slug // tool slug used for other purposes
	}

	body := fmt.Sprintf(`Hey!

Thanks for subscribing to Stockyard %s (%s tier). Here's your license key:

%s

To activate, set this environment variable before starting:

  %s=%s %s

Or export it in your shell profile (~/.bashrc, ~/.zshrc):

  export %s=%s

The key is verified locally — no network call, no phone-home.

Quick links:
- Your Hub: https://stockyard.dev/hub/ (browse and install all tools)
- Your tool page: https://stockyard.dev/%s/
- Docs: https://stockyard.dev/docs
- Support: hello@stockyard.dev
- Manage subscription: https://stockyard.dev/account

If you have any questions, just reply to this email.

— Stockyard
Wrangle your Stack.`, productName, tier, licenseKey, envVar, licenseKey, slug, envVar, licenseKey, slug)

	return m.send(to, subject, body)
}

// SendCancellation sends a cancellation confirmation.
func (m *SMTPMailer) SendCancellation(to, productName string) error {
	subject := fmt.Sprintf("Your %s subscription has been canceled", productName)

	body := fmt.Sprintf(`Hey,

Your %s subscription has been canceled. Your license key will continue
working until the end of your current billing period, then revert to
the free tier (unlimited requests).

Your data and configuration are preserved — just re-subscribe at
stockyard.dev/pricing to pick up where you left off.

We'd love to know what we could do better. Just reply to this email.

— Stockyard`, productName)

	return m.send(to, subject, body)
}

// Send sends a plain-text email via SMTP.
func (m *SMTPMailer) Send(to, subject, body string) error {
	return m.send(to, subject, body)
}

func (m *SMTPMailer) send(to, subject, body string) error {
	if err := ValidateEmail(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	from := m.From
	if from == "" {
		from = "hello@stockyard.dev"
	}

	// Sanitize all header values to prevent header injection
	msg := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n%s",
		sanitizeHeader(m.FromName), sanitizeHeader(from), sanitizeHeader(to), sanitizeHeader(subject), body)

	addr := m.Host + ":" + m.Port
	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send to %s: %w", to, err)
	}

	log.Printf("email: sent %q to %s", subject, to)
	return nil
}

// LogMailer logs emails instead of sending them (for development).
type LogMailer struct{}

// SendLicenseKey logs the license key email.
func (m *LogMailer) SendLicenseKey(to, productName, tier, licenseKey string) error {
	log.Printf("📧 [dev] License key email to %s:", to)
	log.Printf("   Product: %s (%s)", productName, tier)
	log.Printf("   Key: %s", licenseKey)
	log.Printf("   → export STOCKYARD_LICENSE_KEY=%s", licenseKey)
	return nil
}

// SendCancellation logs the cancellation email.
func (m *LogMailer) SendCancellation(to, productName string) error {
	log.Printf("📧 [dev] Cancellation email to %s for %s", to, productName)
	return nil
}

// Send logs the email instead of sending it.
func (m *LogMailer) Send(to, subject, body string) error {
	log.Printf("📧 [dev] Email to %s: %s", to, subject)
	return nil
}

// NewMailer creates the appropriate mailer based on environment.
func NewMailer() Mailer {
	smtp := SMTPMailerFromEnv()
	if smtp != nil {
		return smtp
	}
	// Check for common transactional email services
	if os.Getenv("RESEND_API_KEY") != "" {
		return &ResendMailer{APIKey: os.Getenv("RESEND_API_KEY")}
	}
	log.Printf("⚠️  No email configured (SMTP_HOST or RESEND_API_KEY). Using log mailer.")
	return &LogMailer{}
}

// ResendMailer sends emails via Resend API (popular with indie devs).
type ResendMailer struct {
	APIKey string
}

// SendLicenseKey sends via Resend.
func (m *ResendMailer) SendLicenseKey(to, productName, tier, licenseKey string) error {
	// Determine the correct env var name
	envVar := "STOCKYARD_LICENSE_KEY"
	slug := strings.ToLower(productName)
	slug = strings.ReplaceAll(slug, " ", "")
	if isKnownTool(slug) {
		// All tools use STOCKYARD_LICENSE_KEY (unified env var)
		_ = slug // tool slug used for other purposes
	}
	return m.sendResend(to,
		fmt.Sprintf("Your %s license key", productName),
		fmt.Sprintf("Thanks for subscribing to Stockyard %s (%s)!\n\nYour license key:\n\n%s\n\nActivate with:\n  %s=%s %s\n\nOr export it:\n  export %s=%s\n\nYour Hub: https://stockyard.dev/hub/\nDocs: https://stockyard.dev/docs\n\n— Stockyard\nWrangle your Stack.",
			productName, tier, licenseKey, envVar, licenseKey, slug, envVar, licenseKey),
	)
}

// SendCancellation sends via Resend.
func (m *ResendMailer) SendCancellation(to, productName string) error {
	return m.sendResend(to,
		fmt.Sprintf("Your %s subscription has been canceled", productName),
		fmt.Sprintf("Your %s subscription has been canceled. Your key works until the billing period ends.\n\nRe-subscribe anytime at stockyard.dev/pricing\n\n— Stockyard", productName),
	)
}

// Send sends a plain-text email via Resend.
func (m *ResendMailer) Send(to, subject, body string) error {
	return m.sendResend(to, subject, body)
}

func (m *ResendMailer) sendResend(to, subject, text string) error {
	if err := ValidateEmail(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	payload := fmt.Sprintf(`{"from":"Stockyard <hello@stockyard.dev>","to":["%s"],"subject":"%s","text":"%s"}`,
		escapeJSON(to), escapeJSON(subject), escapeJSON(text))

	req, _ := newHTTPRequest("POST", "https://api.resend.com/emails", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := readAll(resp.Body)
		return fmt.Errorf("resend %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("email: sent %q to %s via Resend", subject, to)
	return nil
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
