// Package billingerr defines billing error types shared between proxy and features
// packages to avoid an import cycle (proxy → features → proxy).
package billingerr

// BillingLimitError is returned when a customer exceeds plan limits (429).
type BillingLimitError struct {
	Msg      string
	Customer string
	Limit    string
}

func (e *BillingLimitError) Error() string { return e.Msg }

// BillingModelError is returned when a customer tries a blocked model (403).
type BillingModelError struct {
	Msg      string
	Customer string
	Model    string
}

func (e *BillingModelError) Error() string { return e.Msg }

// IsBillingLimitError checks if an error is a billing limit violation.
func IsBillingLimitError(err error) (*BillingLimitError, bool) {
	if e, ok := err.(*BillingLimitError); ok {
		return e, true
	}
	return nil, false
}

// IsBillingModelError checks if an error is a billing model access violation.
func IsBillingModelError(err error) (*BillingModelError, bool) {
	if e, ok := err.(*BillingModelError); ok {
		return e, true
	}
	return nil, false
}
