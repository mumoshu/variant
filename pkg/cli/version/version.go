package version

type Version struct {
	FrameworkVersion   string `json:"framework_version"`
	ApplicationVersion string `json:"application_version"`
}

var VERSION string

func Get() (Version, error) {
	return Version{FrameworkVersion: VERSION}, nil
}
