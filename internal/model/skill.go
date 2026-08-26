package model

// SkillKey is hev's host-neutral identity for one logical Skill.
type SkillKey string

// Skill is the logical Skill known by hev Core. Host-specific content, paths,
// providers, and locators remain owned by the host adapter.
type Skill struct {
	Key SkillKey `json:"skillKey"`
}

// SkillPolicyKind selects how a Skill is exposed inside one Environment.
type SkillPolicyKind string

// EnvironmentSkillPolicy is the Environment-specific policy for one Skill.
type EnvironmentSkillPolicy struct {
	Kind SkillPolicyKind `json:"kind"`
}

// EnvironmentSkill records how one Environment uses a logical Skill.
type EnvironmentSkill struct {
	SkillKey SkillKey               `json:"skillKey"`
	Policy   EnvironmentSkillPolicy `json:"policy"`
}
