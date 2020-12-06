package shalm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/Masterminds/semver/v3"
	"github.com/k14s/starlark-go/starlark"
	"github.com/pkg/errors"
	gitignore "github.com/sabhiram/go-gitignore"
	corev1 "k8s.io/api/core/v1"
)

// Chart -
type Chart interface {
	GetGenus() string
	GetName() string
	GetVersion() *semver.Version
	GetNamespace() string
	Apply(thread *starlark.Thread, k K8s) error
	Delete(thread *starlark.Thread, k K8s, options *DeleteOptions) error
	Template(thread *starlark.Thread, k K8s) Stream
	Package(writer io.Writer, helmFormat bool) error
	AddUsedBy(reference string, k K8s) (int, error)
	RemoveUsedBy(reference string, k K8s) (int, error)
}

// ChartValue -
type ChartValue interface {
	Chart
	StructPropertyValue
}

type chartImpl struct {
	ChartOptions
	clazz    chartClass
	Version  semver.Version
	values   starlark.StringDict
	methods  map[string]starlark.Callable
	dir      string
	repo     Repo
	initFunc *starlark.Function
}

var (
	_ ChartValue         = (*chartImpl)(nil)
	_ starlark.HasSetKey = (*chartImpl)(nil)
)

func newChart(thread *starlark.Thread, repo Repo, dir string, opts ...ChartOption) (*chartImpl, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	name := strings.Split(filepath.Base(abs), ":")[0]
	co := chartOptions(opts)
	c := &chartImpl{dir: dir, ChartOptions: *co, clazz: chartClass{Name: name}, repo: repo}
	c.values = make(map[string]starlark.Value)
	c.methods = make(map[string]starlark.Callable)
	hasChartYaml := false
	if err := c.loadChartYaml(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		hasChartYaml = true
	}
	if err := c.loadYaml("values.yaml"); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		hasChartYaml = true
	}
	if err := c.init(thread, hasChartYaml, co); err != nil {
		return nil, err
	}
	if len(co.genus) != 0 {
		c.clazz.Genus = co.genus
	}
	if co.version != nil {
		c.clazz.Version = co.version.String()
	}
	c.SetValue(co.properties.GetValue())
	return c, nil

}

func (c *chartImpl) builtin(name string, fn func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)) starlark.Callable {
	return starlark.NewBuiltin(name+" at "+path.Join(c.dir, "Chart.star"), fn)
}

func (c *chartImpl) GetGenus() string {
	if len(c.clazz.Genus) == 0 {
		return c.clazz.Name
	}
	return c.clazz.Genus
}

func (c *chartImpl) GetName() string {
	if c.suffix == "" {
		return c.clazz.Name
	}
	return fmt.Sprintf("%s-%s", c.clazz.Name, c.suffix)
}

func (c *chartImpl) GetVersion() *semver.Version {
	return c.clazz.GetVersion()
}

func (c *chartImpl) GetNamespace() string {
	return c.namespace
}

func (c *chartImpl) GetVersionString() string {
	return c.clazz.Version
}

func (c *chartImpl) walk(cb func(name string, size int64, body io.Reader, err error) error) error {
	ignore := func(rel string) bool { return strings.HasPrefix(rel, ".") }
	i, err := gitignore.CompileIgnoreFile(path.Join(c.dir, ".shalmignore"))
	if err == nil {
		ignore = func(rel string) bool {
			return rel == ".shalmignore" || i.MatchesPath(rel)
		}
	}
	return filepath.Walk(c.dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(c.dir, file)
		if err != nil {
			return err
		}
		if ignore(rel) {
			return nil
		}
		body, err := os.Open(file)
		if err != nil {
			return err
		}
		defer body.Close()
		return cb(rel, info.Size(), body, nil)
	})
}

func (c *chartImpl) path(part ...string) string {
	return filepath.Join(append([]string{c.dir}, part...)...)
}

func (c *chartImpl) String() string {
	buf := new(strings.Builder)
	buf.WriteString("chart")
	buf.WriteByte('(')
	s := 0
	for i, e := range c.values {
		if s > 0 {
			buf.WriteString(", ")
		}
		s++
		buf.WriteString(i)
		buf.WriteString(" = ")
		buf.WriteString(e.String())
	}
	buf.WriteByte(')')
	return buf.String()
}

// Type -
func (c *chartImpl) Type() string { return "chart" }

// Truth -
func (c *chartImpl) Truth() starlark.Bool { return true } // even when empty

// Hash -
func (c *chartImpl) Hash() (uint32, error) {
	return 0, errors.New("Hash() not implemented")
}

// Freeze -
func (c *chartImpl) Freeze() {
}

// Attr returns the value of the specified field.
func (c *chartImpl) Attr(name string) (starlark.Value, error) {
	if name == "namespace" {
		return starlark.String(c.namespace), nil
	}
	if name == "name" {
		return starlark.String(c.GetName()), nil
	}
	if name == "__class__" {
		return &c.clazz, nil
	}
	value, ok := c.values[name]
	if !ok {
		var m starlark.Value
		m, ok = c.methods[name]
		if !ok {
			m = nil
		}
		if m == nil {
			return nil, starlark.NoSuchAttrError(
				fmt.Sprintf("chart has no .%s attribute", name))

		}
		return m, nil
	}
	property, ok := value.(*property)
	if ok {
		value = property.GetValueOrDefault()
	}
	return WrapDict(value), nil
}

// AttrNames returns a new sorted list of the struct fields.
func (c *chartImpl) AttrNames() []string {
	names := make([]string, 0)
	for k := range c.values {
		names = append(names, k)
	}
	names = append(names, "template")
	return names
}

// SetField -
func (c *chartImpl) SetField(name string, val starlark.Value) error {
	val = UnwrapDict(val)
	existing, ok := c.values[name]
	if ok {
		property, ok := existing.(PropertyValue)
		if ok {
			subchart, ok := val.(Property)
			if ok {
				err := subchart.SetValue(property)
				if err != nil {
					return err
				}
			} else {
				return property.SetValue(val)
			}
		}
	}
	c.values[name] = val
	return nil
}

func (c *chartImpl) Get(name starlark.Value) (starlark.Value, bool, error) {
	value, found := c.values[name.(starlark.String).GoString()]
	if !found {
		return starlark.None, found, nil
	}
	property, ok := value.(*property)
	if ok {
		value = property.GetValueOrDefault()
	}
	return WrapDict(value), true, nil
}

func (c *chartImpl) SetKey(name, value starlark.Value) error {
	c.values[name.(starlark.String).GoString()] = UnwrapDict(value)
	return nil
}

func (c *chartImpl) applyFunction() starlark.Callable {
	return c.builtin("apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		if err := starlark.UnpackArgs("apply", args, kwargs, "k8s", &k); err != nil {
			return nil, err
		}
		return starlark.None, c.apply(thread, k)
	})
}

func (c *chartImpl) Apply(thread *starlark.Thread, k K8s) error {
	_, err := starlark.Call(thread, c.methods["apply"], starlark.Tuple{NewK8sValue(k)}, nil)
	if err != nil {
		return err
	}
	k.Progress(100)
	return nil
}

func (c *chartImpl) apply(thread *starlark.Thread, k K8sValue) error {
	err := c.eachSubChart(func(subChart *chartImpl) error {
		_, err := starlark.Call(thread, subChart.methods["apply"], starlark.Tuple{k}, nil)
		return err
	})
	if err != nil {
		return err
	}
	return c.applyLocal(thread, k, &K8sOptions{ClusterScoped: true}, "")
}

func (c *chartImpl) applyLocalFunction() starlark.Callable {
	return c.builtin("__apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		parser := &kwargsParser{kwargs: kwargs}
		var glob string
		k8sOptions := unpackK8sOptions(parser)
		if err := starlark.UnpackArgs("__apply", args, parser.Parse(), "k8s", &k, "glob?", &glob); err != nil {
			return nil, err
		}
		return starlark.None, c.applyLocal(thread, k, k8sOptions, glob)
	})
}

func (c *chartImpl) applyLocal(thread *starlark.Thread, k K8sValue, k8sOptions *K8sOptions, glob string) error {
	vault := &vaultK8s{k8s: k, namespace: c.namespace}
	err := c.eachJewel(func(v *jewel) error {
		return v.read(vault)
	})
	if err != nil {
		return err
	}
	if c.readOnly {
		return nil
	}
	k8sOptions.ClusterScoped = true
	return k.Apply(decode(c.template(thread, glob, k)), k8sOptions)
}

func (c *chartImpl) objName() string {
	return "shalm." + c.GetGenus()
}

func (c *chartImpl) configMap() *Object {
	return &Object{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "ConfigMap",
		MetaData: MetaData{
			Name:      c.objName(),
			Namespace: c.namespace,
			Labels: map[string]string{
				"shalm.wonderix.github.com/chart":   "true",
				"shalm.wonderix.github.com/genus":   c.GetGenus(),
				"shalm.wonderix.github.com/version": c.GetVersionString(),
			},
			Annotations: map[string]string{
				"kapp.k14s.io/disable-original": "true",
			},
		},
	}
}

func (c *chartImpl) secret() *Object {
	return &Object{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
		MetaData: MetaData{
			Name:      c.objName(),
			Namespace: c.namespace,
			Labels: map[string]string{
				"shalm.wonderix.github.com/chart":   "true",
				"shalm.wonderix.github.com/genus":   c.GetGenus(),
				"shalm.wonderix.github.com/version": c.GetVersionString(),
			},
			Annotations: map[string]string{
				"kapp.k14s.io/disable-original": "true",
			},
		},
	}
}

func (c *chartImpl) modifyConfigMap(obj *Object) error {
	buffer := &bytes.Buffer{}
	if err := c.Package(buffer, false); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{
		"genus":   c.GetGenus(),
		"version": c.GetVersion().String(),
		"chart":   base64.StdEncoding.EncodeToString(buffer.Bytes()),
	})
	if err != nil {
		return err
	}
	obj.Additional = map[string]json.RawMessage{
		"data": data,
	}
	return nil
}
func (c *chartImpl) modifySecret(obj *Object) error {
	byteData := map[string][]byte{}
	// only persist properties
	for _, t := range c.GetValue().(starlark.IterableMapping).Items() {
		j, err := json.Marshal(toGo(t.Index(1)))
		if err != nil {
			return err
		}
		byteData[t.Index(0).(starlark.String).GoString()] = j
	}
	data, err := json.Marshal(byteData)
	if err != nil {
		return err
	}
	obj.Additional = map[string]json.RawMessage{
		"data": data,
	}
	return nil
}

func (c *chartImpl) Delete(thread *starlark.Thread, k K8s, options *DeleteOptions) error {
	thread.SetLocal("delete-options", options)
	_, err := starlark.Call(thread, c.methods["delete"], starlark.Tuple{NewK8sValue(k)}, nil)
	if err != nil {
		return err
	}
	k.Progress(100)
	return nil
}

func (c *chartImpl) deleteFunction() starlark.Callable {
	return c.builtin("delete", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		if err := starlark.UnpackArgs("delete", args, kwargs, "k8s", &k); err != nil {
			return nil, err
		}
		return starlark.None, c.delete(thread, k)
	})
}

func (c *chartImpl) delete(thread *starlark.Thread, k K8sValue) error {
	err := c.eachSubChart(func(subChart *chartImpl) error {
		_, err := starlark.Call(thread, subChart.methods["delete"], starlark.Tuple{k}, nil)
		return err
	})
	if err != nil {
		return err
	}
	return c.deleteLocal(thread, k, &K8sOptions{ClusterScoped: true}, "")
}

func (c *chartImpl) deleteLocalFunction() starlark.Callable {
	return c.builtin("__delete", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var k K8sValue
		var glob string
		parser := &kwargsParser{kwargs: kwargs}
		k8sOptions := unpackK8sOptions(parser)
		if err := starlark.UnpackArgs("__delete", args, parser.Parse(), "k8s", &k, "glob?", &glob); err != nil {
			return nil, err
		}
		return starlark.None, c.deleteLocal(thread, k, k8sOptions, glob)
	})
}

func (c *chartImpl) deleteLocal(thread *starlark.Thread, k K8sValue, k8sOptions *K8sOptions, glob string) error {
	if c.readOnly {
		return nil
	}
	k8sOptions.ClusterScoped = true
	err := k.Delete(decode(c.template(thread, glob, k)), k8sOptions)
	if err != nil {
		return err
	}
	vault := &vaultK8s{k8s: k, namespace: c.namespace}
	return c.eachJewel(func(v *jewel) error {
		return v.delete(vault)
	})
}

func (c *chartImpl) eachSubChart(block func(subChart *chartImpl) error) error {
	for _, v := range c.values {
		subChart, ok := v.(*chartImpl)
		if ok {
			err := block(subChart)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *chartImpl) wrapApply(callable starlark.Callable) starlark.Callable {
	return c.builtin("wrap_apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("Missing first argument k8s")
		}
		k, ok := args[0].(K8sValue)
		if !ok {
			return nil, fmt.Errorf("Invalid first argument to %s", callable.Name())
		}
		for _, v := range c.values {
			dependency, ok := v.(*dependency)
			if ok {
				err := dependency.Apply(thread, k)
				if err != nil {
					return starlark.None, err
				}
			}
		}
		value, err := starlark.Call(thread, callable, args, kwargs)
		if err != nil {
			return value, err
		}
		if !c.skipChart {
			_, err = k.CreateOrUpdate(c.configMap(), c.modifyConfigMap, &K8sOptions{Quiet: true})
			if err != nil {
				return starlark.None, nil
			}
			_, err = k.CreateOrUpdate(c.secret(), c.modifySecret, &K8sOptions{Quiet: true})
			if err != nil {
				return starlark.None, nil
			}
		}
		return value, err

	})
}

func (c *chartImpl) wrapDelete(callable starlark.Callable) starlark.Callable {
	return c.builtin("wrap_delete", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		l := thread.Local("delete-options")
		var deleteOptions *DeleteOptions
		if l != nil {
			deleteOptions = l.(*DeleteOptions)
		} else {
			deleteOptions = &DeleteOptions{}
		}
		if len(args) == 0 {
			return starlark.None, fmt.Errorf("Missing first argument k8s")
		}
		k, ok := args[0].(K8sValue)
		if !ok {
			return starlark.None, fmt.Errorf("Invalid first argument to %s", callable.Name())
		}
		if !deleteOptions.force {
			obj, err := k.Get("configmap", c.objName(), &K8sOptions{IgnoreNotFound: true, Quiet: true})
			if err != nil {
				return starlark.None, err
			}
			if obj != nil {
				if remainingReferences(obj) > 0 {
					return starlark.None, fmt.Errorf("Can't delete %s in namespace %s, because it's still used by other charts", c.GetName(), c.namespace)
				}
			}
		}
		if !c.skipChart {
			for _, obj := range []*Object{c.configMap(), c.secret()} {
				err := k.DeleteByName(obj.Kind, obj.MetaData.Name, &K8sOptions{IgnoreNotFound: true, Quiet: true})
				if err != nil {
					return starlark.None, err
				}
			}
		}
		value, err := starlark.Call(thread, callable, args, kwargs)
		if err != nil {
			return value, err
		}

		for _, v := range c.values {
			dependency, ok := v.(*dependency)
			if ok {
				err := dependency.Delete(thread, k, deleteOptions)
				if err != nil {
					return starlark.None, err
				}
			}
		}
		thread.SetLocal("delete-options", deleteOptions)
		return value, nil

	})
}

func (c *chartImpl) eachJewel(block func(x *jewel) error) error {
	for _, val := range c.values {
		v, ok := val.(*jewel)
		if ok {
			err := block(v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *chartImpl) mergeValues(values map[string]interface{}) {
	for k, v := range values {
		c.values[k] = merge(c.values[k], ToStarlark(v))
	}
}

func (c *chartImpl) getValue(getter func(ReadProperty) starlark.Value) *starlark.Dict {
	result := starlark.NewDict(0)
	for n, val := range c.values {
		property, ok := val.(ReadProperty)
		if ok {
			value := getter(property)
			if value.Truth() {
				result.SetKey(starlark.String(n), value)
			}
		}
	}
	return result
}

func (c *chartImpl) GetValue() starlark.Value {
	return c.getValue(func(p ReadProperty) starlark.Value { return p.GetValue() })
}

func (c *chartImpl) GetValueOrDefault() starlark.Value {
	return c.getValue(func(p ReadProperty) starlark.Value { return p.GetValueOrDefault() })
}

func (c *chartImpl) SetValue(value starlark.Value) error {
	o, ok := value.(ReadProperty)
	if ok {
		value = o.GetValueOrDefault()
	}
	m, ok := value.(starlark.IterableMapping)
	if !ok {
		return fmt.Errorf("value must be a dict not %s", value.String())
	}
	for _, t := range m.Items() {
		key, ok := t.Index(0).(starlark.String)
		if ok {
			err := c.SetField(key.GoString(), t.Index(1))
			if err != nil {
				return err
			}
		}
	}
	return nil

}

func remainingReferences(obj *Object) int {
	counter := 0
	if obj == nil {
		return 0
	}
	for k := range obj.MetaData.Annotations {
		if strings.HasPrefix(k, "shalm-usedby-") {
			counter++
		}
	}
	return counter
}

func usedByAnnotation(reference string) string {
	return "shalm-usedby-" + invalidValueRegex.ReplaceAllString(reference, "-")
}

func (c *chartImpl) AddUsedBy(reference string, k K8s) (int, error) {
	obj, err := k.Patch("configmap", c.objName(), types.JSONPatchType, fmt.Sprintf(`[{"op": "add", "path": "/metadata/annotations/%s", "value" : "True"}]`, usedByAnnotation(reference)), &K8sOptions{Namespace: c.namespace})
	if err != nil {
		return 0, fmt.Errorf("can't add reference to configmap %s in namespace %s: %v %v", c.objName(), c.namespace, err, obj)
	}
	return remainingReferences(obj), nil
}

func (c *chartImpl) RemoveUsedBy(reference string, k K8s) (int, error) {
	obj, err := k.Patch("configmap", c.objName(), types.JSONPatchType, fmt.Sprintf(`[{"op": "remove", "path": "/metadata/annotations/%s"}]`, usedByAnnotation(reference)), &K8sOptions{Namespace: c.namespace, IgnoreNotFound: true})
	if err != nil {
		return 0, fmt.Errorf("can't remove reference from configmap %s in namespace %s: %v", c.objName(), c.namespace, err)
	}
	return remainingReferences(obj), nil
}
