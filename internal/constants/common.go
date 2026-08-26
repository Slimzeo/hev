package constants

const (
	OutputText = "text"
	OutputJSON = "json"

	CLIResponseSchemaVersion      = 2
	EnvironmentStoreSchemaVersion = 1

	BaseEnvironmentID       = "base"
	BaseEnvironmentName     = "base"
	LegacyBaseEnvironmentID = "env_base"

	KebabCasePattern = `^[a-z0-9]+(?:-[a-z0-9]+)*$`

	SkillPolicyAuto      = "auto"
	SkillPolicyOff       = "off"
	DefaultGuideSkillKey = "hev-guide"
)
