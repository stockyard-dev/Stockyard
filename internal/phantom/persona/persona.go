// Package persona provides persistent persona management with memory and habits.
package persona

// Profile defines a phantom persona's behavior characteristics.
type Profile struct {
	Habits      []string `json:"habits"`
	Preferences map[string]string `json:"preferences"`
	Patterns    []string `json:"patterns"`
}

// MemoryEntry represents accumulated context from past sessions.
type MemoryEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	SessionID string `json:"session_id"`
}
