package variant

type Input struct {
	Name          string `yaml:"name,omitempty"`
	Description   string `yaml:"description,omitempty"`
	ArgumentIndex *int   `yaml:"argument-index,omitempty"`
}
