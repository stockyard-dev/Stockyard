package features

import "sync"

// AuditFunc matches Trust.Auditor() signature: (eventType, actor, resource, action, detail)
type AuditFunc func(string, string, string, string, any)

var (
	auditMu  sync.RWMutex
	auditFn  AuditFunc
)

// SetAuditFunc installs the trust auditor for use by middlewares.
// Called from engine wiring after trust app is initialized.
func SetAuditFunc(fn AuditFunc) {
	auditMu.Lock()
	auditFn = fn
	auditMu.Unlock()
}

// reportAudit fires an audit event through the trust auditor if wired.
// This ensures all ledger writes go through Trust.RecordEvent's mutex,
// maintaining hash chain integrity.
func reportAudit(eventType, actor, resource, action string, detail any) {
	auditMu.RLock()
	fn := auditFn
	auditMu.RUnlock()
	if fn != nil {
		fn(eventType, actor, resource, action, detail)
	}
}
