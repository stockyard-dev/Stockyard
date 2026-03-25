// Package conflict provides conflict detection when memories contradict.
package conflict

// Type represents the kind of conflict between two memories.
type Type string

const (
	Contradiction Type = "contradiction"
	Superseded    Type = "superseded"
	Ambiguous     Type = "ambiguous"
)

// Detect checks if two memory values about the same subject conflict.
func Detect(subject string, oldValue, newValue string, oldConf, newConf float64) (Type, bool) {
	if oldValue == newValue { return "", false }
	if newConf > oldConf+0.2 { return Superseded, true }
	if oldConf > newConf+0.2 { return Superseded, true }
	return Contradiction, true
}
