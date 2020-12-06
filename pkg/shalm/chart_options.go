package shalm

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/k14s/starlark-go/starlark"
	"github.com/spf13/pflag"
)

// Properties -
type Properties struct {
	dict *starlark.Dict
}

func (p Properties) String() string {
	return `""`
}

var setVarRegexp = regexp.MustCompile("^([a-z][a-z0-9_]*)=(.*)")

func parseSet(val string, cb func(key string, value string) error) error {
	match := setVarRegexp.FindStringSubmatch(val)
	if match == nil {
		return fmt.Errorf("Invalid argument %s", val)
	}
	if err := cb(match[1], match[2]); err != nil {
		return err
	}
	return nil
}

func (p *Properties) get(key string) starlark.Value {
	if p.dict == nil {
		return starlark.None
	}
	v, found, err := p.dict.Get(starlark.String(key))
	if err != nil {
		panic(err)
	}
	if !found {
		return starlark.None
	}
	return v
}

func (p *Properties) delete(key string) starlark.Value {
	if p.dict == nil {
		return starlark.None
	}
	v, found, err := p.dict.Delete(starlark.String(key))
	if err != nil {
		panic(err)
	}
	if !found {
		return starlark.None
	}
	return v
}

func (p *Properties) set(key string, value starlark.Value) {
	if p.dict == nil {
		p.dict = starlark.NewDict(0)
	}
	p.dict.SetKey(starlark.String(key), value)
}

func (p *Properties) setWithMap(values map[string]interface{}) {
	for k, v := range values {
		p.set(k, ToStarlark(v))
	}
}

// GetValue -
func (p *Properties) GetValue() starlark.Value {
	if p.dict == nil {
		p.dict = starlark.NewDict(0)
	}
	return p.dict
}

// Set -
func (p *Properties) Set(val string) error {
	return parseSet(val, func(key string, value string) error {
		p.set(key, starlark.String(value))
		return nil
	})
}

// Type -
func (p Properties) Type() string {
	return "properties"
}

type propertiesYamlVar struct {
	properties *Properties
}

func (p propertiesYamlVar) String() string {
	return p.properties.String()
}

// Set -
func (p *propertiesYamlVar) Set(val string) error {
	return parseSet(val, func(key string, value string) error {
		var v map[string]interface{}
		err := readYamlFile(value, &v)
		if err != nil {
			return err
		}
		p.properties.set(key, ToStarlark(v))
		return nil
	})
}

// Type -
func (p propertiesYamlVar) Type() string {
	return "properties-yaml"
}

// proeprtiesFileVar -
type proeprtiesFileVar struct {
	properties *Properties
}

func (p proeprtiesFileVar) String() string {
	return p.properties.String()
}

// Set -
func (p *proeprtiesFileVar) Set(val string) error {
	return parseSet(val, func(key string, value string) error {
		data, err := ioutil.ReadFile(value)
		if err != nil {
			return err
		}
		p.properties.set(key, starlark.String(string(data)))
		return nil
	})
}

// Type -
func (p proeprtiesFileVar) Type() string {
	return "properties-file"
}

type propertiesEnvVar struct {
	properties *Properties
}

func (p propertiesEnvVar) String() string {
	return p.properties.String()
}

// Set -
func (p *propertiesEnvVar) Set(val string) error {
	return parseSet(val, func(key string, value string) error {
		p.properties.set(key, starlark.String(os.Getenv(value)))
		return nil
	})
}

// Type -
func (p propertiesEnvVar) Type() string {
	return "properties-env"
}

type propertiesFile struct {
	properties *Properties
}

func (p propertiesFile) String() string {
	return p.properties.String()
}

// Set -
func (p *propertiesFile) Set(val string) error {
	var values map[string]interface{}
	err := readYamlFile(val, &values)
	if err != nil {
		return err
	}
	p.properties.setWithMap(values)
	return nil
}

// Type -
func (p propertiesFile) Type() string {
	return "values"
}

type GenusAndVersion struct {
	genus   string
	version *semver.Version
}

func NewGenusAndVersion(url string) *GenusAndVersion {
	var match []string
	if match = githubRelease.FindStringSubmatch(url); match != nil {
		return extractGenusAndVersion(match[1], match[2])
	}
	if match = githubArchive.FindStringSubmatch(url); match != nil {
		return extractGenusAndVersion(match[1], match[2])
	}
	if match = githubEnterpriseArchive.FindStringSubmatch(url); match != nil {
		return extractGenusAndVersion(match[1]+"/"+match[2], match[3])
	}
	if match = otherURL.FindStringSubmatch(url); match != nil {
		return extractGenusAndVersion(match[2], match[3])
	}
	if match = catalogURL.FindStringSubmatch(url); match != nil {
		return extractGenusAndVersion(match[1], "")
	}
	return &GenusAndVersion{}
}

// ChartOptions -
type ChartOptions struct {
	GenusAndVersion
	namespace  string
	suffix     string
	args       starlark.Tuple
	properties Properties
	skipChart  bool
	readOnly   bool
}

// ChartOption -
type ChartOption func(options *ChartOptions)

func (s *GenusAndVersion) AsOptions() []ChartOption {
	result := make([]ChartOption, 0)
	if len(s.genus) != 0 {
		result = append(result, WithGenus(s.genus))
	}
	if s.version != nil {
		result = append(result, WithVersion(s.version))
	}
	return result
}

// WithNamespace -
func WithNamespace(namespace string) ChartOption {
	return func(options *ChartOptions) { options.namespace = namespace }
}

// WithGenus -
func WithGenus(value string) ChartOption {
	return func(options *ChartOptions) { options.genus = value }
}

// WithVersion -
func WithVersion(value *semver.Version) ChartOption {
	return func(options *ChartOptions) { options.version = value }
}

// WithSuffix -
func WithSuffix(suffix string) ChartOption {
	return func(options *ChartOptions) { options.suffix = suffix }
}

// WithArgs -
func WithArgs(args starlark.Tuple) ChartOption {
	return func(options *ChartOptions) { options.args = args }
}

// WithProperties -
func WithProperties(properties *starlark.Dict) ChartOption {
	return func(options *ChartOptions) { options.properties = Properties{dict: properties} }
}

// WithKwArgs -
func WithKwArgs(kwargs []starlark.Tuple) ChartOption {
	return func(options *ChartOptions) {
		for _, arg := range kwargs {
			if arg.Len() == 2 {
				key, keyOK := arg.Index(0).(starlark.String)
				if keyOK {
					options.properties.set(key.GoString(), arg.Index(1))
				}
			}
		}
	}
}

// WithValues -
func WithValues(values map[string]interface{}) ChartOption {
	return func(options *ChartOptions) { options.properties.setWithMap(values) }
}

// WithSkipChart -
func WithSkipChart(value bool) ChartOption {
	return func(options *ChartOptions) { options.skipChart = value }
}

// WithReadOnly -
func WithReadOnly(value bool) ChartOption {
	return func(options *ChartOptions) { options.readOnly = value }
}

// AddFlags -
func (v *ChartOptions) AddFlags(flagsSet *pflag.FlagSet) {
	defaultNamespace := os.Getenv("SHALM_NAMESPACE")
	if defaultNamespace == "" {
		defaultNamespace = "default"
	}
	flagsSet.Var(&v.properties, "set", "Set values (key=val).")
	flagsSet.Var(&propertiesYamlVar{properties: &v.properties}, "set-yaml", "Set values from respective YAML files (key=path).")
	flagsSet.Var(&proeprtiesFileVar{properties: &v.properties}, "set-file", "Set values from respective files (key=path).")
	flagsSet.Var(&propertiesEnvVar{properties: &v.properties}, "set-env", "Set values from respective environment variable (key=env).")
	flagsSet.StringVarP(&v.namespace, "namespace", "n", defaultNamespace, "namespace for installation")
	flagsSet.StringVarP(&v.suffix, "suffix", "s", "", "Suffix which is used to build the chart name")
	flagsSet.VarP(&propertiesFile{properties: &v.properties}, "values", "f", "Load additional values from a file")
}

func (v *ChartOptions) KwArgs(f *starlark.Function) []starlark.Tuple {
	result := []starlark.Tuple{}
	for i := 1; i < f.NumParams(); i++ {
		arg, _ := f.Param(i)
		value := v.properties.delete(arg)
		if value != starlark.None {
			result = append(result, starlark.Tuple{starlark.String(arg), value})
		}
	}
	return result
}

// Merge -
func (v *ChartOptions) Merge() ChartOption {
	return func(o *ChartOptions) {
		*o = *v
	}
}

func chartOptions(opts []ChartOption) *ChartOptions {
	co := ChartOptions{}
	for _, option := range opts {
		option(&co)
	}
	if co.namespace == "" {
		co.namespace = "default"
	}
	return &co
}
