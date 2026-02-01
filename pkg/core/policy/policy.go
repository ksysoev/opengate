// Package policy defines types for middleware policy configuration.
package policy

// Policy represents a configured middleware instance.
// It is created from the policies section in config.yml and referenced by routes in OpenAPI specs.
type Policy struct {
	Config map[string]interface{}
	Name   string
	Type   string
}

// PolicyDefinition represents a single policy entry from config.yml.
type PolicyDefinition struct {
	Config map[string]interface{} `mapstructure:"config"`
	Type   string                 `mapstructure:"type"`
}

// PolicyConfig represents the policies section in config.yml.
// It maps policy names to their definitions.
type PolicyConfig map[string]PolicyDefinition

// ToPolicies converts PolicyConfig to a map of Policy instances.
func (pc PolicyConfig) ToPolicies() map[string]Policy {
	policies := make(map[string]Policy, len(pc))
	for name, def := range pc {
		policies[name] = Policy{
			Name:   name,
			Type:   def.Type,
			Config: def.Config,
		}
	}

	return policies
}
