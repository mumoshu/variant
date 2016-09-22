package engine

type Parameter struct {
	Name     string `yaml:"name,omitempty"`
	Value    string `yaml:"value,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`
}
