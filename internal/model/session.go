package model

// Session is hev's resolved Environment state for one host Session.
// Environment is nil when hev is not activated.
type Session struct {
	Source      Source       `json:"source"`
	SessionID   string       `json:"sessionId"`
	Environment *Environment `json:"environment"`
}
