package features

import "sync"

// SafetyReportFunc is called by safety middlewares to record events.
// Args: eventType, severity, category, actionTaken, model, requestID, sourceIP, userID, detail
type SafetyReportFunc func(string, string, string, string, string, string, string, string, any)

var (
	safetyMu       sync.RWMutex
	safetyReporter SafetyReportFunc
)

// SetSafetyReporter installs the global safety reporter (called from engine wiring).
func SetSafetyReporter(fn SafetyReportFunc) {
	safetyMu.Lock()
	safetyReporter = fn
	safetyMu.Unlock()
}

// reportSafety fires a safety event if the reporter is wired.
func reportSafety(eventType, severity, category, actionTaken, model, requestID, sourceIP, userID string, detail any) {
	safetyMu.RLock()
	fn := safetyReporter
	safetyMu.RUnlock()
	if fn != nil {
		go fn(eventType, severity, category, actionTaken, model, requestID, sourceIP, userID, detail)
	}
}
