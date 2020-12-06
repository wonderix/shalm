package shalm

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/k14s/starlark-go/starlark"
)

// K8sValue -
type K8sValue interface {
	starlark.Value
	K8s
}

// NewK8sValue create new instance to interact with kubernetes
func NewK8sValue(k K8s) K8sValue {
	result, ok := k.(K8sValue)
	if ok {
		return result
	}
	return &k8sValueImpl{k}
}

type k8sValueImpl struct {
	K8s
}

type k8sWatcher struct {
	k8s     K8s
	kind    string
	name    string
	options *K8sOptions
}

type k8sWatcherIterator struct {
	next   chan *Object
	cancel chan struct{}
}

var (
	_ starlark.Iterable    = (*k8sWatcher)(nil)
	_ starlark.Iterator    = (*k8sWatcherIterator)(nil)
	_ K8sValue             = (*k8sValueImpl)(nil)
	_ starlark.HasSetField = (*k8sValueImpl)(nil)
)

func makeK8sValue(k8s K8s, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
	var kubeconfig string
	if err := starlark.UnpackArgs("k8s", args, kwargs, "kubeconfig", &kubeconfig); err != nil {
		return starlark.None, err
	}
	newK8s, err := k8s.ForConfig(kubeconfig)
	if err != nil {
		return starlark.None, err
	}
	return &k8sValueImpl{newK8s}, nil
}

// String -
func (k *k8sValueImpl) String() string { return k.Inspect() }

// Type -
func (k *k8sValueImpl) Type() string { return "k8s" }

// Freeze -
func (k *k8sValueImpl) Freeze() {}

// Truth -
func (k *k8sValueImpl) Truth() starlark.Bool { return true }

// Hash -
func (k *k8sValueImpl) Hash() (uint32, error) { panic("implement me") }

// Attr -
func (k *k8sValueImpl) Attr(name string) (starlark.Value, error) {
	switch name {
	case "rollout_status":
		{
			return starlark.NewBuiltin("rollout_status", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("rollout_status", args, parser.Parse(),
					"kind", &kind, "name", &name); err != nil {
					return nil, err
				}
				return starlark.None, k.RolloutStatus(kind, name, k8sOptions)
			}), nil
		}
	case "wait":
		{
			return starlark.NewBuiltin("wait", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				var condition string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("wait", args, parser.Parse(),
					"kind", &kind, "name", &name, "condition", &condition); err != nil {
					return nil, err
				}
				return starlark.None, k.Wait(kind, name, condition, k8sOptions)
			}), nil
		}
	case "delete":
		{
			return starlark.NewBuiltin("delete", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("delete", args, parser.Parse(),
					"kind", &kind, "name?", &name); err != nil {
					return nil, err
				}
				if name == "" {
					return starlark.None, errors.New("no parameter name given")
				}
				return starlark.None, k.DeleteObject(kind, name, k8sOptions)
			}), nil
		}
	case "apply":
		{
			return starlark.NewBuiltin("apply", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
				var value starlark.Value
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("apply", args, parser.Parse(), "value", &value); err != nil {
					return nil, err
				}
				var os func(w ObjectWriter) error
				s, ok := value.(*stream)
				if ok {
					os = decode(s.Stream)
				} else {
					os = func(w ObjectWriter) error {
						var o Object
						data, err := json.Marshal(toGo(value))
						if err != nil {
							return err
						}
						if err = json.Unmarshal(data, &o); err != nil {
							return err
						}
						return w(&o)
					}
				}
				return starlark.None, k.Apply(os, k8sOptions)
			}), nil
		}
	case "get":
		{
			return starlark.NewBuiltin("get", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("get", args, parser.Parse(),
					"kind", &kind, "name", &name); err != nil {
					return nil, err
				}
				if name == "" {
					return starlark.None, errors.New("no parameter name given")
				}
				obj, err := k.Get(kind, name, k8sOptions)
				if err != nil {
					return starlark.None, err
				}
				if obj == nil {
					return starlark.None, nil
				}
				return WrapDict(ToStarlark(obj)), nil
			}), nil
		}
	case "patch":
		{
			return starlark.NewBuiltin("patch", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				typ := string(types.JSONPatchType)
				var patch string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("get", args, parser.Parse(),
					"kind", &kind, "name", &name, "patch", &patch, "type?", &typ); err != nil {
					return nil, err
				}
				if name == "" {
					return starlark.None, errors.New("no parameter name given")
				}
				obj, err := k.Patch(kind, name, types.PatchType(typ), patch, k8sOptions)
				if err != nil {
					return starlark.None, err
				}
				return WrapDict(ToStarlark(obj)), nil
			}), nil
		}
	case "list":
		{
			return starlark.NewBuiltin("list", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("list", args, parser.Parse(),
					"kind", &kind); err != nil {
					return nil, err
				}
				obj, err := k.List(kind, k8sOptions, &ListOptions{})
				if err != nil {
					return starlark.None, err
				}
				return WrapDict(ToStarlark(obj)), nil
			}), nil
		}
	case "watch":
		{
			return starlark.NewBuiltin("watch", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				var kind string
				var name string
				parser := &kwargsParser{kwargs: kwargs}
				k8sOptions := unpackK8sOptions(parser)
				if err := starlark.UnpackArgs("get", args, parser.Parse(),
					"kind", &kind, "name", &name); err != nil {
					return nil, err
				}
				if name == "" {
					return starlark.None, errors.New("no parameter name given")
				}
				return &k8sWatcher{name: name, kind: kind, options: k8sOptions, k8s: k.K8s}, nil
			}), nil
		}
	case "for_config":
		{
			return starlark.NewBuiltin("for_config", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
				return makeK8sValue(k, args, kwargs)
			}), nil

		}
	case "progress":
		return k.progressFunction()
	case "host":
		return starlark.String(k.Host()), nil
	case "tool":
		return starlark.String(k.Tool().String()), nil

	}

	return starlark.None, starlark.NoSuchAttrError(fmt.Sprintf("k8s has no .%s attribute", name))
}

// SetField -
func (k *k8sValueImpl) SetField(name string, val starlark.Value) error {
	switch name {
	case "tool":
		var tool Tool
		if err := tool.Set(val.(starlark.String).GoString()); err != nil {
			return err
		}
		k.SetTool(tool)
		return nil
	}
	return starlark.NoSuchAttrError(fmt.Sprintf("k8s has no .%s attribute", name))
}

func (k *k8sValueImpl) progressFunction() (starlark.Callable, error) {
	return starlark.NewBuiltin("progress", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, e error) {
		var progress int
		if err := starlark.UnpackArgs("progress", args, kwargs, "progress", &progress); err != nil {
			return starlark.None, err
		}
		k.Progress(progress)
		return starlark.None, nil
	}), nil
}

// AttrNames -
func (k *k8sValueImpl) AttrNames() []string {
	return []string{"rollout_status", "delete", "get", "wait", "for_config", "host", "tool"}
}

func unpackK8sOptions(parser *kwargsParser) *K8sOptions {
	result := &K8sOptions{}
	parser.Arg("namespaced", func(value starlark.Value) {
		result.ClusterScoped = !bool(value.(starlark.Bool))
	})
	parser.Arg("ignore_not_found", func(value starlark.Value) {
		result.IgnoreNotFound = bool(value.(starlark.Bool))
	})
	parser.Arg("namespace", func(value starlark.Value) {
		result.Namespace = value.(starlark.String).GoString()
	})
	parser.Arg("timeout", func(value starlark.Value) {
		timeout, ok := value.(starlark.Int).Int64()
		if ok {
			result.Timeout = time.Duration(timeout) * time.Second
		}
	})
	return result
}

func (w *k8sWatcher) Freeze()               {}
func (w *k8sWatcher) String() string        { return "k8sWatcher" }
func (w *k8sWatcher) Type() string          { return "k8sWatcher" }
func (w *k8sWatcher) Truth() starlark.Bool  { return true }
func (w *k8sWatcher) Hash() (uint32, error) { return 0, fmt.Errorf("k8sWatcher is unhashable") }
func (w *k8sWatcher) Iterate() starlark.Iterator {
	i := &k8sWatcherIterator{next: make(chan *Object, 1), cancel: make(chan struct{}, 1)}
	go func() {
		stream := w.k8s.Watch(w.kind, w.name, w.options)
		writer := func(obj *Object) error {
			select {
			case <-i.cancel:
				return &CancelObjectStream{}
			case i.next <- obj:
				return nil
			}
		}
		stream(writer)
	}()
	return i
}

func (i *k8sWatcherIterator) Next(p *starlark.Value) bool {
	obj := <-i.next
	*p = WrapDict(ToStarlark(obj))
	return true
}

func (i *k8sWatcherIterator) Done() {
	i.cancel <- struct{}{}
}
