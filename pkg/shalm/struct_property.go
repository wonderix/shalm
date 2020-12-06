package shalm

import (
	"errors"
	"fmt"

	"github.com/k14s/starlark-go/starlark"
)

type structProperty struct {
	properties           map[string]PropertyValue
	additionalProperties bool
}

type StructPropertyValue interface {
	Property
	starlark.HasSetField
}

var _ starlark.HasSetKey = (*structProperty)(nil)
var _ StructPropertyValue = (*structProperty)(nil)

func newStructProperty(additionalProperties bool) *structProperty {
	return &structProperty{properties: make(map[string]PropertyValue), additionalProperties: additionalProperties}
}

func makeStructProperty(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
	s := newStructProperty(false)

	if len(args) != 0 {
		return starlark.None, fmt.Errorf("%s: got %d arguments, want at most %d", "struct_property", len(args), 0)
	}
	for _, arg := range kwargs {
		if arg.Len() == 2 {
			key, keyOK := arg.Index(0).(starlark.String)
			if keyOK {
				if key.GoString() == "additional_properties" {
					s.additionalProperties = bool(arg.Index(1).Truth())
				} else {
					property, ok := arg.Index(1).(PropertyValue)
					if !ok {
						return starlark.None, fmt.Errorf("%s: invalid argument type. Must be property", "struct_property")
					}
					s.add(key.GoString(), property)
				}
			}
		}
	}
	return s, nil
}

func (s *structProperty) add(name string, property PropertyValue) {
	s.properties[name] = property
}

func (s *structProperty) String() string {
	return fmt.Sprintf("struct_property(properties = %v)", s.properties)
}

func (s *structProperty) Type() string {
	return "struct_property"
}

func (s *structProperty) Freeze() {
}

func (s *structProperty) Truth() starlark.Bool {
	return len(s.properties) != 0
}

func (s *structProperty) Hash() (uint32, error) {
	return 0, errors.New("Hash() not implemented")
}

func (s *structProperty) Attr(name string) (starlark.Value, error) {
	p, ok := s.properties[name]
	if !ok {
		return starlark.None, starlark.NoSuchAttrError(fmt.Sprintf("struct_property has no .%s property", name))
	}
	simpleProperty, ok := p.(*property)
	if ok {
		return simpleProperty.GetValueOrDefault(), nil
	}
	return p, nil
}

func (s *structProperty) SetValue(value starlark.Value) error {
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
			err := s.SetField(key.GoString(), t.Index(1))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *structProperty) getValue(getter func(ReadProperty) starlark.Value) *starlark.Dict {
	result := starlark.NewDict(len(s.properties))
	for name, property := range s.properties {
		value := getter(property)
		if value.Truth() {
			result.SetKey(starlark.String(name), value)
		}
	}
	return result
}

func (s *structProperty) GetValue() starlark.Value {
	return s.getValue(func(p ReadProperty) starlark.Value { return p.GetValue() })
}

func (s *structProperty) GetValueOrDefault() starlark.Value {
	return s.getValue(func(p ReadProperty) starlark.Value { return p.GetValueOrDefault() })
}

// SetField -
func (s *structProperty) SetField(name string, val starlark.Value) error {
	property, ok := s.properties[name]
	if !ok {
		if !s.additionalProperties {
			return starlark.NoSuchAttrError(fmt.Sprintf("struct_property has no .%s property", name))
		}
		_, ok := val.(starlark.IterableMapping)
		if ok {
			property = newStructProperty(s.additionalProperties)
		} else {
			property = newProperty(starlark.None)
		}
		s.properties[name] = property
	}
	err := property.SetValue(val)
	if err != nil {
		return err
	}
	return nil
}

func (s *structProperty) SetKey(k, v starlark.Value) error {
	return s.SetField(k.(starlark.String).GoString(), v)
}

func (s *structProperty) Get(name starlark.Value) (starlark.Value, bool, error) {
	value, err := s.Attr(name.(starlark.String).GoString())
	if err != nil {
		_, ok := err.(starlark.NoSuchAttrError)
		if ok {
			return starlark.None, false, nil
		}
		return starlark.None, false, err
	}
	return value, true, nil
}

func (s *structProperty) AttrNames() []string {
	keys := make([]string, 0, len(s.properties))
	for k := range s.properties {
		keys = append(keys, k)
	}
	return keys
}
