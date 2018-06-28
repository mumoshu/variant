package variant

type Input struct {
	Name          string               `yaml:"name,omitempty"`
	Parameters    map[string]Parameter `yaml:"parameters,omitempty"`
	Description   string               `yaml:"description,omitempty"`
	Candidates    []string             `yaml:"candidates,omitempty"`
	Complete      string               `yaml:"complete,omitempty"`
	ArgumentIndex *int                 `yaml:"argument-index,omitempty"`
}
