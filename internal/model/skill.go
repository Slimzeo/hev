package model

// SkillKey is hev's host-neutral identity for one logical Skill.
type SkillKey string

// Skill is the logical Skill known by hev Core. Host-specific content, paths,
// providers, and locators remain owned by the host adapter.
type Skill struct {
	Key SkillKey `json:"skillKey"`
}

// Validate verifies the Skill identity owned by hev Core.
func (s Skill) Validate() error {
	if !keyPattern.MatchString(string(s.Key)) {
		return NewError(StatusCodeInvalidArgument, "invalid skill key %q: use lowercase kebab-case", s.Key)
	}
	return nil
}

// SkillPolicyKind selects how a Skill is exposed inside one Environment.
type SkillPolicyKind string

const (
	SkillPolicyAuto SkillPolicyKind = "auto"
	SkillPolicyOff  SkillPolicyKind = "off"
)

// EnvironmentSkillPolicy is the Environment-specific policy for one Skill.
type EnvironmentSkillPolicy struct {
	Kind SkillPolicyKind `json:"kind"`
}

// Validate rejects policy values not implemented by hev.
func (p EnvironmentSkillPolicy) Validate() error {
	switch p.Kind {
	case SkillPolicyAuto, SkillPolicyOff:
		return nil
	default:
		return NewError(StatusCodeInvalidArgument, "unsupported skill policy: %s", p.Kind)
	}
}

// EnvironmentSkill records how one Environment uses a logical Skill.
type EnvironmentSkill struct {
	SkillKey SkillKey               `json:"skillKey"`
	Policy   EnvironmentSkillPolicy `json:"policy"`
}
