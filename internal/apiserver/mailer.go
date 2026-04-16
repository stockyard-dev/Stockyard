package apiserver

import (
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"
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
	SendBundleTrialKey(to, bundleName, bundleSlug, licenseKey, trialEndDate string, tools []string) error
	SendTrialReminder(to, bundleName string, daysLeft int) error
	SendTrialConverted(to, bundleName string) error
	SendCancellation(to, productName string) error
	// Desktop trial flow (added Apr 16 2026 for the 7-day card-capture trial).
	SendDesktopTrialKey(to, tier, licenseKey string, trialEndUnix int64) error
	SendDesktopLicenseConverted(to, tier, licenseKey string) error
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
	subject := fmt.Sprintf("Your Stockyard %s license key", productName)

	body := fmt.Sprintf(`Hey,

Thanks for subscribing to Stockyard %s (%s). Your license key is below.

Your license key:

  %s

To activate, set this environment variable before starting the tool:

  export STOCKYARD_LICENSE_KEY=%s

The key is verified on your computer. Nothing gets sent anywhere. Once your tool reads it, you are good.

If you need help getting this running, just reply to this email. I read every message myself, usually within a few hours. A friendlier setup flow is on the way, but for now a short Terminal session is how it works.

Heads up: Windows is not supported yet. If you are on Windows, reply and I will let you know as soon as it is.

Your data stays on your machine forever. No cloud. No data ever leaves your computer. If you cancel, you keep everything.

Michael
hello@stockyard.dev`, productName, tier, licenseKey, licenseKey)

	return m.send(to, subject, body)
}

// SendBundleTrialKey sends the welcome email for a bundle trial signup. This
// is the email a non-technical buyer sees first after paying, so it leads with
// the key, admits the install is a Terminal command today, and offers explicit
// reply-based help for anyone who is not comfortable opening Terminal.
func (m *SMTPMailer) SendBundleTrialKey(to, bundleName, bundleSlug, licenseKey, trialEndDate string, tools []string) error {
	subject := fmt.Sprintf("Welcome to Stockyard for %s", bundleName)

	toolList := strings.Join(tools, ", ")
	if toolList == "" {
		toolList = "(the full bundle)"
	}
	trialLine := "Trial ends " + trialEndDate + ". After that it is $7.99/mo, and you can cancel anytime by replying to this email."
	if trialEndDate == "" {
		trialLine = "You are on a 14-day free trial. After that it is $7.99/mo, and you can cancel anytime by replying to this email."
	}

	body := fmt.Sprintf(`Hey,

Thanks for starting a trial of Stockyard for %s. Your license key is below and I am around if you need any help getting it running.

Your license key:

  %s

%s

Tools you get with this bundle: %s.

Here is how to install it on a Mac or Linux computer. If you are comfortable with Terminal, the whole install is two commands:

  curl -fsSL https://stockyard.dev/for/%s/install.sh | sh
  export STOCKYARD_LICENSE_KEY=%s

Each tool in your bundle runs as its own small program on your computer. Once install finishes, reply to this email and tell me which one you want to try first. I will send you the exact line to run and where to click once it opens.

If you have never opened Terminal, do not worry about it. Reply to this email and tell me what kind of computer you have. I will walk you through it over email or jump on a quick call. A friendlier one-click installer is on the way, but I did not want to make you wait on it to get started.

Heads up on Windows: it is not supported yet. If you are on Windows, reply and I will let you know the day it is.

Your data stays on your machine forever. No cloud. No data ever leaves your computer. If you cancel, you keep everything.

Any questions at all, just reply. I read every message myself.

Michael
hello@stockyard.dev`, bundleName, licenseKey, trialLine, toolList, bundleSlug, licenseKey)

	return m.send(to, subject, body)
}

// SendTrialReminder sends a trial reminder email.
func (m *SMTPMailer) SendTrialReminder(to, bundleName string, daysLeft int) error {
	var subject, body string

	switch {
	case daysLeft == 7:
		subject = fmt.Sprintf("How's Stockyard for %s working?", bundleName)
		body = `You are halfway through your trial. Quick question: is Stockyard doing what you need it to?

If yes, great. You do not need to do anything. It will continue at $7.99/mo after your trial.

If something is missing, hit reply and tell me. I can usually build what you need within a day or two.

7 days left in your trial.

Michael
hello@stockyard.dev`

	case daysLeft == 2:
		subject = "Your Stockyard trial ends in 2 days"
		body = fmt.Sprintf(`Just a heads up, your Stockyard for %s trial ends in 2 days.

After that, your subscription starts at $7.99/mo. You do not need to do anything. It is automatic.

If you want to cancel, just reply to this email and I will take care of it.

Your data stays on your machine no matter what. We never delete anything.

Michael
hello@stockyard.dev`, bundleName)

	default:
		subject = fmt.Sprintf("Stockyard trial: %d days remaining", daysLeft)
		body = fmt.Sprintf(`Your Stockyard for %s trial has %d days remaining.

Your data stays on your machine forever, whether you subscribe or not.

Michael
hello@stockyard.dev`, bundleName, daysLeft)
	}

	return m.send(to, subject, body)
}

// SendTrialConverted sends confirmation when trial converts to paid.
func (m *SMTPMailer) SendTrialConverted(to, bundleName string) error {
	subject := fmt.Sprintf("Your Stockyard subscription is active. %s", bundleName)
	body := fmt.Sprintf(`Your trial is over and your %s subscription has started.

$7.99/mo. Cancel anytime by replying to this email. Your data is always yours.

Tip: back up your data by copying the /data folder. That is it. Your entire system is in that folder.

Thanks for choosing Stockyard.

Michael
hello@stockyard.dev`, bundleName)

	return m.send(to, subject, body)
}

// SendCancellation sends a cancellation confirmation.
func (m *SMTPMailer) SendCancellation(to, productName string) error {
	subject := fmt.Sprintf("Your %s subscription has been canceled", productName)

	body := fmt.Sprintf(`Hey,

Your %s subscription has been canceled. Your license key will continue
working until the end of your current billing period, then revert to
the free tier (unlimited requests).

Your data and configuration are preserved. Re-subscribe anytime at
stockyard.dev/pricing to pick up where you left off.

If there is something I could do better, just reply to this email and
tell me. I read every message myself.

Michael
hello@stockyard.dev`, productName)

	return m.send(to, subject, body)
}

// Send sends a plain-text email via SMTP.
func (m *SMTPMailer) Send(to, subject, body string) error {
	return m.send(to, subject, body)
}

// SendDesktopTrialKey sends the activation email for a 7-day desktop trial.
// Fired from handleCheckoutCompleted right after Stripe checkout completes.
//
// tier is the eventual paid tier the customer chose at checkout
// ("local", "cloud-single", "cloud-multi"). The license itself is
// minted with tier=trial; this string is for human-readable copy
// ("After day 7 you'll be charged for your Cloud Single Site plan").
func (m *SMTPMailer) SendDesktopTrialKey(to, tier, licenseKey string, trialEndUnix int64) error {
	tierName := desktopTierDisplayName(tier)
	trialEnd := time.Unix(trialEndUnix, 0).UTC().Format("January 2, 2006")

	subject := "Your Stockyard Desktop trial is active"
	body := fmt.Sprintf(`Hey,

Welcome to Stockyard Desktop. Your 7-day free trial is live until %s.
After that, your card will be charged for the %s plan you selected.

Your license key:

  %s

Save this key — drop the .stockyard-license file into the app, or
paste the key into the License panel (top-right corner of the app).

If you decide it's not for you, cancel anytime before %s in your
billing portal: https://stockyard.dev/billing/

Reply to this email if anything's broken or confusing — I read every
message myself.

Michael
hello@stockyard.dev`, trialEnd, tierName, licenseKey, trialEnd)

	return m.send(to, subject, body)
}

// SendDesktopLicenseConverted sends the "you're now a customer" email
// when the trial converts to paid on day 7. Includes the new permanent
// license key (different key — the trial one expires).
func (m *SMTPMailer) SendDesktopLicenseConverted(to, tier, licenseKey string) error {
	tierName := desktopTierDisplayName(tier)

	subject := "Your Stockyard Desktop license — welcome aboard"
	body := fmt.Sprintf(`Hey,

Your trial just converted. You're officially a Stockyard Desktop
%s customer — thank you.

Your permanent license key:

  %s

Drop this into the app to replace your trial license. Same place as
before: License panel in the top-right corner, or save the
.stockyard-license file to your Stockyard data directory.

A few things worth knowing:
  - Your old trial license still works until it expires; no rush.
  - This permanent license has no expiry. If Stockyard the company
    disappears, your binary keeps working forever.
  - Manage billing anytime: https://stockyard.dev/billing/

Reply to this email if anything's wrong.

Michael
hello@stockyard.dev`, tierName, licenseKey)

	return m.send(to, subject, body)
}

// desktopTierDisplayName converts an internal tier string into the
// customer-facing label used in trial/conversion emails.
func desktopTierDisplayName(tier string) string {
	switch tier {
	case "local":
		return "Local (one-time, $99)"
	case "cloud-single":
		return "Cloud Single Site"
	case "cloud-multi":
		return "Cloud Multi-Site"
	}
	return tier
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

// SendBundleTrialKey logs the bundle trial email.
func (m *LogMailer) SendBundleTrialKey(to, bundleName, bundleSlug, licenseKey, trialEndDate string, tools []string) error {
	log.Printf("📧 [dev] Bundle trial email to %s: %s (%d tools, trial ends %s)", to, bundleName, len(tools), trialEndDate)
	log.Printf("   Key: %s", licenseKey)
	return nil
}

// SendTrialReminder logs the trial reminder email.
func (m *LogMailer) SendTrialReminder(to, bundleName string, daysLeft int) error {
	log.Printf("📧 [dev] Trial reminder to %s: %s (%d days left)", to, bundleName, daysLeft)
	return nil
}

// SendTrialConverted logs the trial conversion email.
func (m *LogMailer) SendTrialConverted(to, bundleName string) error {
	log.Printf("📧 [dev] Trial converted email to %s: %s", to, bundleName)
	return nil
}

// Send logs the email instead of sending it.
func (m *LogMailer) Send(to, subject, body string) error {
	log.Printf("📧 [dev] Email to %s: %s", to, subject)
	return nil
}

// SendDesktopTrialKey logs the desktop trial activation email.
func (m *LogMailer) SendDesktopTrialKey(to, tier, licenseKey string, trialEndUnix int64) error {
	log.Printf("📧 [dev] Desktop trial activation to %s: tier=%s ends=%s",
		to, tier, time.Unix(trialEndUnix, 0).Format("2006-01-02"))
	log.Printf("   Key: %s", licenseKey)
	return nil
}

// SendDesktopLicenseConverted logs the trial-to-paid conversion email.
func (m *LogMailer) SendDesktopLicenseConverted(to, tier, licenseKey string) error {
	log.Printf("📧 [dev] Desktop trial converted to paid for %s: tier=%s", to, tier)
	log.Printf("   Key: %s", licenseKey)
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

// SendLicenseKey sends via Resend. Voice: conversational first-person, no em
// dashes, admits the install reality (Terminal required for now), and explicitly
// offers reply-based support for non-dev buyers.
func (m *ResendMailer) SendLicenseKey(to, productName, tier, licenseKey string) error {
	body := fmt.Sprintf(`Hey,

Thanks for subscribing to Stockyard %s (%s). Your license key is below.

Your license key:

  %s

To activate, set this environment variable before starting the tool:

  export STOCKYARD_LICENSE_KEY=%s

The key is verified on your computer. Nothing gets sent anywhere. Once your tool reads it, you're good.

If you need help getting this running, just reply to this email. I read every message myself, usually within a few hours. A friendlier setup flow is on the way, but for now a short Terminal session is how it works.

Heads up: Windows is not supported yet. If you are on Windows, reply and I will let you know as soon as it is.

Your data stays on your machine forever. No cloud. No data ever leaves your computer. If you cancel, you keep everything.

Michael
hello@stockyard.dev`,
		productName, tier, licenseKey, licenseKey)

	return m.sendResend(to, fmt.Sprintf("Your Stockyard %s license key", productName), body)
}

// SendCancellation sends via Resend.
func (m *ResendMailer) SendCancellation(to, productName string) error {
	return m.sendResend(to,
		fmt.Sprintf("Your %s subscription has been canceled", productName),
		fmt.Sprintf(`Hey,

Your %s subscription has been canceled. You will not be billed again.

Your license key will keep working until the end of the current billing period, so your tools continue to run normally until then. After that, the tools keep running but will show a "license expired" notice. Your data stays on your machine forever either way. Nothing gets deleted. Nothing gets sent anywhere. Nothing ever leaves your computer.

If you want to come back later, you can resubscribe anytime at https://stockyard.dev/pricing and pick up where you left off.

If you canceled by mistake, or if you canceled because something was broken or frustrating, please reply and tell me what happened. I read every message myself and I would genuinely like to know what went wrong.

Michael
hello@stockyard.dev`, productName),
	)
}

// SendBundleTrialKey sends the welcome email for a bundle trial signup. This
// is the email a non-technical buyer sees first after paying, so it leads with
// the key, admits the install is a Terminal command today, and offers explicit
// reply-based help for anyone who is not comfortable opening Terminal.
func (m *ResendMailer) SendBundleTrialKey(to, bundleName, bundleSlug, licenseKey, trialEndDate string, tools []string) error {
	toolList := strings.Join(tools, ", ")
	if toolList == "" {
		toolList = "every Stockyard tool"
	}
	trialLine := "Your trial ends " + trialEndDate + ". After that it is $7.99/mo, and you can cancel anytime by replying to this email."
	if trialEndDate == "" {
		trialLine = "You are on a 14-day free trial. After that it is $7.99/mo, and you can cancel anytime by replying to this email."
	}

	body := fmt.Sprintf(`Hey,

Thanks for starting a trial of %s. I am around if you need any help getting it running. Just reply to this email.

%s

Your license key (you will need this in step 2 below):

%s

What is included: %s.

How to install on a Mac or Linux computer
------------------------------------------

If you are comfortable with Terminal, copy and paste these two lines, one at a time. The first line downloads and installs your tools. The second line activates your license so the tools know you are a paying customer.

1. Install:

   curl -fsSL https://stockyard.dev/for/%s/install.sh | sh

2. Activate (paste your license key from above where it says YOUR_KEY):

   export STOCKYARD_LICENSE_KEY=YOUR_KEY

After both commands run, your tools are installed in ~/stockyard-%s/ on your computer. Reply to this email and tell me which tool you want to open first. I will send you the exact line to run and where to click once it opens in your browser.

If you have never opened Terminal before
-----------------------------------------

Do not worry about any of that. Reply to this email and tell me what kind of computer you have (Mac, Linux, or Windows) and I will walk you through it over email or hop on a quick call. A friendlier one-click installer is on the way, but I did not want to make you wait on it to get started.

Heads up on Windows: it is not supported yet. If you are on Windows, reply and I will let you know the day it is ready.

Your data stays on your machine forever. No cloud. No data ever leaves your computer. If you cancel, you keep everything you have entered.

Any questions at all, just reply. I read every message myself.

Michael
hello@stockyard.dev`,
		bundleName, trialLine, licenseKey, toolList, bundleSlug, bundleSlug)

	return m.sendResend(to, fmt.Sprintf("Welcome to %s", bundleName), body)
}

func (m *ResendMailer) SendTrialReminder(to, bundleName string, daysLeft int) error {
	subject := fmt.Sprintf("Stockyard trial: %d days remaining", daysLeft)
	body := fmt.Sprintf("Your Stockyard for %s trial has %d days remaining.\n\nYour data stays on your machine forever.\n\nMichael\nhello@stockyard.dev", bundleName, daysLeft)
	return m.sendResend(to, subject, body)
}

func (m *ResendMailer) SendTrialConverted(to, bundleName string) error {
	return m.sendResend(to,
		fmt.Sprintf("Your Stockyard subscription is active. %s", bundleName),
		fmt.Sprintf("Your %s subscription has started. $7.99/mo. Cancel anytime.\n\nTip: back up your data by copying the /data folder.\n\nMichael\nhello@stockyard.dev", bundleName),
	)
}

// Send sends a plain-text email via Resend.
func (m *ResendMailer) Send(to, subject, body string) error {
	return m.sendResend(to, subject, body)
}

func (m *ResendMailer) SendDesktopTrialKey(to, tier, licenseKey string, trialEndUnix int64) error {
	tierName := desktopTierDisplayName(tier)
	trialEnd := time.Unix(trialEndUnix, 0).UTC().Format("January 2, 2006")
	subject := "Your Stockyard Desktop trial is active"
	body := fmt.Sprintf(`Hey,

Welcome to Stockyard Desktop. Your 7-day free trial is live until %s.
After that, your card will be charged for the %s plan you selected.

Your license key:

  %s

Save this key — drop the .stockyard-license file into the app, or
paste the key into the License panel (top-right corner of the app).

If you decide it's not for you, cancel anytime before %s in your
billing portal: https://stockyard.dev/billing/

Reply to this email if anything's broken or confusing.

Michael
hello@stockyard.dev`, trialEnd, tierName, licenseKey, trialEnd)
	return m.sendResend(to, subject, body)
}

func (m *ResendMailer) SendDesktopLicenseConverted(to, tier, licenseKey string) error {
	tierName := desktopTierDisplayName(tier)
	subject := "Your Stockyard Desktop license — welcome aboard"
	body := fmt.Sprintf(`Hey,

Your trial just converted. You're officially a Stockyard Desktop
%s customer — thank you.

Your permanent license key:

  %s

Drop this into the app to replace your trial license. Same place as
before: License panel in the top-right corner, or save the
.stockyard-license file to your Stockyard data directory.

A few things worth knowing:
  - Your old trial license still works until it expires; no rush.
  - This permanent license has no expiry. If Stockyard the company
    disappears, your binary keeps working forever.
  - Manage billing anytime: https://stockyard.dev/billing/

Reply to this email if anything's wrong.

Michael
hello@stockyard.dev`, tierName, licenseKey)
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
