package model

// Source identifies the Coding Agent platform that owns an Environment or Session.
type Source string

const (
	SourceStandalone Source = "standalone"
	SourceDSH        Source = "dsh"
	SourceClaudeCode Source = "claude-code"
	SourceCodex      Source = "codex"
	SourceOpenCode   Source = "opencode"
)

// Valid reports whether Source is supported by hev Core.
func (source Source) Valid() bool {
	switch source {
	case SourceStandalone, SourceDSH, SourceClaudeCode, SourceCodex, SourceOpenCode:
		return true
	default:
		return false
	}
}
