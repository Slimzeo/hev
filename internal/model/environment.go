package model

// EnvironmentID is the stable identifier of an Environment.
type EnvironmentID string

// Environment is the persisted aggregate managed by hev.
type Environment struct {
	Source   Source             `json:"source"`
	ID       EnvironmentID      `json:"id"`
	Name     string             `json:"name"`
	Revision uint64             `json:"revision"`
	Skills   []EnvironmentSkill `json:"skills"`
}
