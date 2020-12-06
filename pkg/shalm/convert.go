package shalm

import (
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/k14s/starlark-go/starlark"
	"github.com/k14s/starlark-go/starlarkstruct"
)

// ToStarlark -
func ToStarlark(vi interface{}) starlark.Value {
	if vi == nil {
		return starlark.None
	}
	switch v := reflect.ValueOf(vi); v.Kind() {
	case reflect.String:
		return starlark.String(v.String())
	case reflect.Bool:
		return starlark.Bool(v.Bool())
	case reflect.Int:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Int16:
		return starlark.MakeInt64(v.Int())
	case reflect.Uint:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Uint16:
		return starlark.MakeUint64(v.Uint())
	case reflect.Float32:
		return starlark.Float(v.Float())
	case reflect.Float64:
		return starlark.Float(v.Float())
	case reflect.Slice:
		if b, ok := vi.([]byte); ok {
			return starlark.String(string(b))
		}
		a := make([]starlark.Value, 0)
		for i := 0; i < v.Len(); i++ {
			a = append(a, ToStarlark(v.Index(i).Interface()))
		}
		return starlark.Tuple(a)
	case reflect.Ptr:
		return ToStarlark(v.Elem().Interface())
	case reflect.Map:
		d := starlark.NewDict(16)
		for _, key := range v.MapKeys() {
			strct := v.MapIndex(key)
			keyValue := ToStarlark(key.Interface())
			d.SetKey(keyValue, ToStarlark(strct.Interface()))
		}
		return d
	case reflect.Struct:
		ios, ok := vi.(intstr.IntOrString)
		if ok {
			switch ios.Type {
			case intstr.String:
				return starlark.String(ios.StrVal)
			case intstr.Int:
				return starlark.MakeInt(int(ios.IntVal))
			}
		} else {
			data, err := json.Marshal(vi)
			if err != nil {
				panic(err)
			}
			var m map[string]interface{}
			err = json.Unmarshal(data, &m)
			if err != nil {
				panic(err)
			}
			return ToStarlark(m)
		}
	}
	panic(fmt.Errorf("cannot convert %v to starlark", vi))
}

// ToGoMap -
func ToGoMap(v starlark.IterableMapping) map[string]interface{} {
	d := make(map[string]interface{})
	for _, t := range v.Items() {
		key, ok := t.Index(0).(starlark.String)
		if ok {
			value := toGo(t.Index(1))
			if value != nil {
				d[key.GoString()] = value
			}
		}
	}
	return d
}

func toGoStringList(v starlark.Value) []string {
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case starlark.Indexable: // Tuple, List
		a := make([]string, 0)
		for i := 0; i < starlark.Len(v); i++ {
			a = append(a, v.Index(i).(starlark.String).GoString())
		}
		return a
	default:
		panic(fmt.Errorf("cannot convert %s to go string list", v.Type()))
	}
}

func toGo(v starlark.Value) interface{} {
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(v)
	case starlark.Int:
		i, _ := v.Int64()
		return i
	case starlark.Float:
		return float64(v)
	case starlark.String:
		return v.GoString()
	case starlark.Indexable: // Tuple, List
		a := make([]interface{}, 0)
		for i := 0; i < starlark.Len(v); i++ {
			a = append(a, toGo(v.Index(i)))
		}
		return a
	case starlark.Callable:
		return nil
	case starlark.IterableMapping:
		return ToGoMap(v)
	case *chartImpl:
		return stringDictToGo(v.values)
	case *stream:
		return nil
	case *dependency:
		return nil
	case *injectedFiles:
		return nil
	case *jewel:
		return v.templateValues()
	case *starlarkstruct.Struct:
		d := starlark.StringDict{}
		v.ToStringDict(d)
		return stringDictToGo(d)
	case Property:
		return toGo(v.GetValueOrDefault())
	default:
		panic(fmt.Errorf("cannot convert %s to go", v.Type()))
	}
}

func stringDictToGo(stringDict starlark.StringDict) map[string]interface{} {
	d := make(map[string]interface{})

	for k, v := range stringDict {
		value := toGo(v)
		if value != nil {
			d[k] = value
		}
	}
	return d
}

func toIntOrString(v starlark.Value) (intstr.IntOrString, error) {
	switch v := v.(type) {
	case starlark.Int:
		i, _ := v.Int64()
		return intstr.FromInt(int(i)), nil
	case starlark.String:
		return intstr.FromString(v.GoString()), nil
	}
	return intstr.IntOrString{}, fmt.Errorf("cannot convert %s to IntOrString", v.String())
}

func toIntOrStringArray(v starlark.Tuple) ([]intstr.IntOrString, error) {
	result := make([]intstr.IntOrString, v.Len())
	for i := 0; i < v.Len(); i++ {
		ios, err := toIntOrString(v.Index(i))
		if err != nil {
			return nil, err
		}
		result[i] = ios
	}
	return result, nil
}

func mergeStringDict(value starlark.StringDict, override starlark.IterableMapping) starlark.StringDict {
	d := starlark.StringDict{}
	for _, t := range override.Items() {
		d[t.Index(0).(starlark.String).GoString()] = t.Index(1)
	}

	for k, v := range value {
		o, found := d[k]
		if found {
			value := merge(v, o)
			if value != nil && value != starlark.None {
				d[k] = value
			}
		} else {
			d[k] = v
		}
	}
	return d
}

func merge(value starlark.Value, override starlark.Value) starlark.Value {
	if override == nil {
		return value
	}
	switch override := override.(type) {
	case starlark.NoneType:
		return value
	case starlark.Bool:
		return override
	case starlark.Int:
		return override
	case starlark.Float:
		return override
	case starlark.String:
		return override
	case starlark.Indexable:
		var result []starlark.Value
		v := value.(starlark.Indexable)
		for i := 0; i < maxInt(override.Len(), v.Len()); i++ {
			if i >= override.Len() {
				result = append(result, v.Index(i))
			} else if i >= v.Len() {
				result = append(result, override.Index(i))
			} else {
				result = append(result, merge(v.Index(i), override.Index(i)))
			}
		}
		_, ok := override.(starlark.Tuple)
		if ok {
			return starlark.Tuple(result)
		}
		return starlark.NewList(result)
	case starlark.IterableMapping:
		switch value := value.(type) {
		case starlark.IterableMapping:
			d := starlark.NewDict(starlark.Len(override))
			for _, t := range override.Items() {
				d.SetKey(t.Index(0), t.Index(1))
			}

			for _, t := range value.Items() {
				key := t.Index(0)
				o, found, err := d.Get(key)
				if found && err == nil {
					value := merge(t.Index(1), o)
					if value != nil && value != starlark.None {
						d.SetKey(key, value)
					}
				} else {
					d.SetKey(key, t.Index(1))
				}
			}
			return d
		case *chartImpl:
			value.values = mergeStringDict(value.values, override)
			return value
		case *starlarkstruct.Struct:
			d := starlark.StringDict{}
			value.ToStringDict(d)
			return starlarkstruct.FromStringDict(starlarkstruct.Default, mergeStringDict(d, override))
		case starlark.NoneType:
			return override
		default:
			panic(fmt.Errorf("cannot merge %s", value.Type()))
		}
	default:
		panic(fmt.Errorf("cannot merge %s", override.Type()))
	}
}

func maxInt(i1, i2 int) int {
	if i1 > i2 {
		return i1
	}
	return i2
}
