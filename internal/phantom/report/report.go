// Package report provides anomaly detection and bug report generation.
package report

// AnomalyReport represents a detected anomaly.
type AnomalyReport struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
}

// Detect checks if an expected vs actual pair constitutes an anomaly.
func Detect(expected, actual string) *AnomalyReport {
	if expected == actual { return nil }
	return &AnomalyReport{
		Severity: "warning", Category: "behavior_change",
		Description: "Response deviated from expectation",
		Expected: expected, Actual: actual,
	}
}
