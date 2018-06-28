package variant

type InputConfig struct {
	Name          string `yaml:"name,omitempty"`
	Description   string `yaml:"description,omitempty"`
	ArgumentIndex *int   `yaml:"argument-index,omitempty"`
}

type ParameterConfig struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type OptionConfig struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}
