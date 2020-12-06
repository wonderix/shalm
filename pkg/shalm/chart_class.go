package shalm

import (
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/k14s/starlark-go/starlark"
	"github.com/pkg/errors"
)

var nameRegexp = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9\\-\\./]*$")

type chartClass struct {
	APIVersion    string                   `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Name          string                   `json:"name,omitempty" yaml:"name,omitempty"`
	Genus         string                   `json:"genus,omitempty" yaml:"genus,omitempty"`
	Version       string                   `json:"version,omitempty" yaml:"version,omitempty"`
	Description   string                   `json:"description,omitempty" yaml:"description,omitempty"`
	Keywords      []string                 `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Home          string                   `json:"home,omitempty" yaml:"home,omitempty"`
	Sources       []string                 `json:"sources,omitempty" yaml:"sources,omitempty"`
	Icon          string                   `json:"icon,omitempty" yaml:"icon,omitempty"`
	KubeVersion   string                   `json:"kubeVersion,omitempty" yaml:"kubeVersion,omitempty"`
	Maintainers   []map[string]interface{} `json:"maintainers,omitempty" yaml:"maintainers,omitempty"`
	Engine        string                   `json:"engine,omitempty" yaml:"engine,omitempty"`
	AppVersion    string                   `json:"appVersion,omitempty" yaml:"appVersion,omitempty"`
	Deprecated    bool                     `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	TillerVersion string                   `json:"tillerVersion,omitempty" yaml:"tillerVersion,omitempty"`
}

var _ starlark.HasSetField = (*chartClass)(nil)

// String -
func (cc *chartClass) String() string { return cc.Name }

// Type -
func (cc *chartClass) Type() string { return "chart_class" }

// Freeze -
func (cc *chartClass) Freeze() {}

// Truth -
func (cc *chartClass) Truth() starlark.Bool { return true }

// Hash -
func (cc *chartClass) Hash() (uint32, error) { panic("implement me") }

// Attr -
func (cc *chartClass) Attr(name string) (starlark.Value, error) {
	switch name {
	case "api_version":
		return starlark.String(cc.APIVersion), nil
	case "name":
		return starlark.String(cc.Name), nil
	case "version":
		return starlark.String(cc.Version), nil
	case "description":
		return starlark.String(cc.Description), nil
	case "keywords":
		return ToStarlark(cc.Keywords), nil
	case "home":
		return starlark.String(cc.Home), nil
	case "sources":
		return ToStarlark(cc.Sources), nil
	case "icon":
		return starlark.String(cc.Icon), nil
	case "kube_version":
		return starlark.String(cc.KubeVersion), nil
	case "maintainers":
		return ToStarlark(cc.Maintainers), nil
	case "engine":
		return starlark.String(cc.Engine), nil
	case "app_version":
		return starlark.String(cc.AppVersion), nil
	case "deprecated":
		return starlark.Bool(cc.Deprecated), nil
	case "tiller_version":
		return starlark.String(cc.TillerVersion), nil
	}
	return starlark.None, starlark.NoSuchAttrError(fmt.Sprintf("chart_class has no .%s attribute", name))
}

// AttrNames -
func (cc *chartClass) AttrNames() []string {
	return []string{"api_version", "name", "version", "description", "keywords", "home", "sources", "icon"}
}

func validateName(name string) error {
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("Invalid name '%s' for chart", name)
	}
	return nil
}
func validateVersion(version string) error {
	if len(version) == 0 {
		return nil
	}
	_, err := semver.NewVersion(version)
	return err
}

func (cc *chartClass) Validate() error {
	if err := validateName(cc.Name); err != nil {
		return err
	}
	if err := validateVersion(cc.Version); err != nil {
		return err
	}
	return nil
}

func (cc *chartClass) GetVersion() *semver.Version {
	if len(cc.Version) == 0 {
		return &semver.Version{}
	}
	result, err := semver.NewVersion(cc.Version)
	if err != nil {
		panic(errors.Wrap(err, "Invalid version in helm chart"))
	}
	return result
}

// SetField -
func (cc *chartClass) SetField(name string, val starlark.Value) error {
	switch name {
	case "api_version":
		cc.APIVersion = val.(starlark.String).GoString()
	case "name":
		name := val.(starlark.String).GoString()
		if err := validateName(name); err != nil {
			return err
		}
		cc.Name = name
	case "version":
		version := val.(starlark.String).GoString()
		if err := validateVersion(version); err != nil {
			return err
		}
		cc.Version = version
	case "description":
		cc.Description = val.(starlark.String).GoString()
	case "keywords":
		cc.Keywords = toGoStringList(val)
	case "home":
		cc.Home = val.(starlark.String).GoString()
	case "sources":
		cc.Sources = toGoStringList(val)
	case "icon":
		cc.Icon = val.(starlark.String).GoString()
	case "kube_version":
		cc.KubeVersion = val.(starlark.String).GoString()
	case "maintainers":
		maintainers := toGo(val).([]interface{})
		cc.Maintainers = make([]map[string]interface{}, 0)
		for _, maintainer := range maintainers {
			cc.Maintainers = append(cc.Maintainers, maintainer.(map[string]interface{}))
		}
	case "engine":
		cc.Engine = val.(starlark.String).GoString()
	case "app_version":
		cc.AppVersion = val.(starlark.String).GoString()
	case "deprecated":
		cc.Deprecated = bool(val.(starlark.Bool))
	case "tiller_version":
		cc.TillerVersion = val.(starlark.String).GoString()
	default:
		return starlark.NoSuchAttrError(fmt.Sprintf("chart_class has no .%s attribute", name))
	}
	return nil
}
