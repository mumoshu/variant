package version

type Version struct {
	FrameworkVersion   string `json:"framework_version"`
	ApplicationVersion string `json:"application_version"`
}

func Get() (Version, error) {
	return Version{FrameworkVersion: "0.0.3-rc.4"}, nil
}
