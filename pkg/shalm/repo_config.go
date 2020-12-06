package shalm

type credential struct {
	URL      string `yaml:"url,omitempty"`
	Token    string `yaml:"token,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type repoConfigs struct {
	Credentials []credential `yaml:"credentials,omitempty"`
	Catalogs    []string     `yaml:"catalogs,omitempty"`
}

// RepoConfig -
type RepoConfig func(r *repoConfigs) error

// WithTokenAuth -
func WithTokenAuth(url string, token string) RepoConfig {
	return func(r *repoConfigs) error {
		r.Credentials = append(r.Credentials, credential{URL: url, Token: token})
		return nil
	}
}

// WithBasicAuth -
func WithBasicAuth(url string, username string, password string) RepoConfig {
	return func(r *repoConfigs) error {
		r.Credentials = append(r.Credentials, credential{URL: url, Username: username, Password: password})
		return nil
	}
}

// WithCatalog -
func WithCatalog(url string) RepoConfig {
	return func(r *repoConfigs) error {
		r.Catalogs = append(r.Catalogs, url)
		return nil
	}
}

// WithConfigFile -
func WithConfigFile(filename string) RepoConfig {
	return func(r *repoConfigs) error {
		return readYamlFile(filename, r)
	}
}
