package shalm

import (
	"errors"
	"fmt"

	"github.com/k14s/starlark-go/starlark"
)

// ReadProperty -
type ReadProperty interface {
	GetValue() starlark.Value
	GetValueOrDefault() starlark.Value
}

// Property -
type Property interface {
	ReadProperty
	SetValue(value starlark.Value) error
}

// PropertyValue -
type PropertyValue interface {
	Property
	starlark.HasAttrs
}

type property struct {
	typ   string
	value starlark.Value
	dflt  starlark.Value
}

var _ PropertyValue = (*property)(nil)

func newProperty(dflt starlark.Value) *property {
	return &property{value: starlark.None, typ: "string", dflt: dflt}
}

func makeProperty(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
	s := newProperty(starlark.None)
	var err error
	if err = starlark.UnpackArgs("property", args, kwargs, "type?", &s.typ, "default?", &s.dflt); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *property) String() string {
	return fmt.Sprintf("property(type = %s, value = %v , default = %v)", s.typ, s.value, s.dflt)
}

func (s *property) Type() string {
	return "property"
}

func (s *property) Freeze() {
}

func (s *property) Truth() starlark.Bool {
	return s.value.Truth()
}

func (s *property) Hash() (uint32, error) {
	return 0, errors.New("Hash() not implemented")
}

func (s *property) Attr(name string) (starlark.Value, error) {
	return starlark.None, starlark.NoSuchAttrError(fmt.Sprintf("property has no .%s attribute", name))
}

func (s *property) SetValue(value starlark.Value) error {
	o, ok := value.(ReadProperty)
	if ok {
		value = o.GetValueOrDefault()
	}
	s.value = value
	return nil
}

func (s *property) GetValue() starlark.Value {
	return s.value
}

func (s *property) GetValueOrDefault() starlark.Value {
	if s.value == starlark.None {
		return s.dflt
	}
	return s.value
}

func (s *property) AttrNames() []string {
	return []string{}
}
