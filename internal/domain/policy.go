package domain

// SkillPolicyKind selects how a skill is exposed inside one environment.
type SkillPolicyKind string

const (
	SkillPolicyAuto SkillPolicyKind = "auto"
	SkillPolicyOff  SkillPolicyKind = "off"
)

// EnvironmentSkillPolicy is the environment-specific policy for one skill.
type EnvironmentSkillPolicy struct {
	Kind SkillPolicyKind `json:"kind"`
}

// Validate rejects policy values not implemented by the MVP.
func (p EnvironmentSkillPolicy) Validate() error {
	switch p.Kind {
	case SkillPolicyAuto, SkillPolicyOff:
		return nil
	default:
		return NewError(ErrorCodeInvalidArgument, "unsupported skill policy: %s", p.Kind)
	}
}
