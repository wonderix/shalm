package shalm

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/k14s/starlark-go/starlark"
)

type dependency struct {
	repo       Repo
	properties StructPropertyValue
	url        string
	constraint *semver.Constraints
	namespace  string
	userBy     func() string
}

var _ starlark.HasAttrs = (*dependency)(nil)
var _ starlark.HasSetField = (*dependency)(nil)
var _ Property = (*dependency)(nil)

func makeDependency(userBy func() string, repo Repo, namespace string) func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {

	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		s := &dependency{properties: newStructProperty(true), namespace: namespace, repo: repo, userBy: userBy}
		var err error
		var constraint string
		if err = starlark.UnpackArgs("dependency", args, kwargs, "url", &s.url, "constraint", &constraint, "namespace?", &s.namespace); err != nil {
			return nil, err
		}
		s.constraint, err = semver.NewConstraint(constraint)
		if err != nil {
			return starlark.None, err
		}
		return s, nil
	}
}

func (s *dependency) String() string {
	return fmt.Sprintf("dependency(url = %s, constrait = %s, properties = %v)", s.url, s.constraint, s.properties)
}

func (s *dependency) Type() string {
	return "dependency"
}

func (s *dependency) Freeze() {
}

func (s *dependency) Truth() starlark.Bool {
	return true
}

func (s *dependency) Hash() (uint32, error) {
	panic("implement me")
}

func (s *dependency) Attr(name string) (starlark.Value, error) {
	return s.properties.Attr(name)
}

func (s *dependency) SetField(name string, val starlark.Value) error {
	return s.properties.SetField(name, val)
}

func (s *dependency) AttrNames() []string {
	return append(s.properties.AttrNames(), "apply")
}

func (s *dependency) GetValue() starlark.Value {
	return s.properties.GetValue()
}

func (s *dependency) GetValueOrDefault() starlark.Value {
	return s.properties.GetValueOrDefault()
}

func (s *dependency) SetValue(value starlark.Value) error {
	return s.properties.SetValue(value)
}

func (s *dependency) resolve(properties StructPropertyValue) error {
	err := properties.SetValue(s.properties.GetValue())
	if err != nil {
		return err
	}
	s.properties = properties
	return nil
}

func (s *dependency) Apply(thread *starlark.Thread, k8s K8s) error {
	_, ok := s.properties.(ChartValue)
	if ok {
		return nil
	}
	gv := NewGenusAndVersion(s.url)
	charts, err := s.repo.List(thread, k8s, &RepoListOptions{namespace: s.namespace, genus: gv.genus})
	if err != nil {
		return err
	}
	if len(charts) > 1 {
		return fmt.Errorf("found more than one chart for genus %s in namespace %s", gv.genus, s.namespace)
	}
	if len(charts) == 1 {
		if !s.constraint.Check(charts[0].GetVersion()) {
			return fmt.Errorf("installed version of genus %s in namespace %s doesn't match constraint %v", gv.genus, s.namespace, s.constraint)
		}
		_, err := charts[0].AddUsedBy(s.userBy(), k8s)
		if err != nil {
			return err
		}
		return s.resolve(charts[0])
	}
	chart, err := s.repo.Get(thread, s.url, append(gv.AsOptions(), WithNamespace(s.namespace))...)
	if err != nil {
		return err
	}
	err = s.resolve(chart)
	if err != nil {
		return err
	}
	err = chart.Apply(thread, k8s)
	if err != nil {
		return err
	}
	_, err = chart.AddUsedBy(s.userBy(), k8s)
	if err != nil {
		return err
	}
	return nil
}

func (s *dependency) Delete(thread *starlark.Thread, k8s K8s, deleteOptions *DeleteOptions) error {
	_, ok := s.properties.(ChartValue)
	if ok {
		return nil
	}
	gv := NewGenusAndVersion(s.url)
	charts, err := s.repo.List(thread, k8s, &RepoListOptions{namespace: s.namespace, genus: gv.genus})
	if err != nil {
		return err
	}
	if len(charts) > 1 {
		return fmt.Errorf("found more than one chart for genus %s in namespace %s", gv.genus, s.namespace)
	}
	if len(charts) == 1 {
		references, err := charts[0].RemoveUsedBy(s.userBy(), k8s)
		if deleteOptions.recursive {
			if references == 0 {
				charts[0].Delete(thread, k8s, deleteOptions)
			}
		}
		return err
	}
	return nil
}
