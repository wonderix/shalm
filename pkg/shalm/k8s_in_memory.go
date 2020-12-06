package shalm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

// K8sInMemory in memory implementation of K8s
type K8sInMemory struct {
	namespace string
	objects   map[string]Object
}

type notFoundError string

func (e notFoundError) Error() string {
	return string(e)
}

// NewK8sInMemory creates a new K8sInMemory instance
func NewK8sInMemory(namespace string, objects ...Object) *K8sInMemory {
	result := &K8sInMemory{namespace: namespace, objects: map[string]Object{}}
	for _, obj := range objects {
		result.objects[result.key(obj.Kind, obj.MetaData.Name, obj.MetaData.Namespace, nil)] = obj
	}
	return result
}

// NewK8sInMemoryEmpty creates a new K8sInMemory instance
func NewK8sInMemoryEmpty() *K8sInMemory {
	return NewK8sInMemory("default")
}

var _ K8s = (*K8sInMemory)(nil)

// Host -
func (k K8sInMemory) Host() string {
	return "memory.local"
}

// ForSubChart -
func (k K8sInMemory) ForSubChart(namespace string, app string, version *semver.Version, children int) K8s {
	return &K8sInMemory{namespace: namespace, objects: k.objects}
}

// WithContext -
func (k K8sInMemory) WithContext(ctx context.Context) K8s {
	return &K8sInMemory{namespace: k.namespace, objects: k.objects}
}

// Inspect -
func (k K8sInMemory) Inspect() string {
	return k.namespace
}

// Tool -
func (k K8sInMemory) Tool() Tool {
	return ToolKubectl
}

// SetTool -
func (k K8sInMemory) SetTool(tool Tool) {
}

// Watch -
func (k K8sInMemory) Watch(kind string, name string, options *K8sOptions) ObjectStream {
	obj, err := k.GetObject(kind, name, options)
	if err != nil {
		return ObjectErrorStream(err)
	}
	return func(writer ObjectWriter) error {
		return writer(obj)
	}
}

// RolloutStatus -
func (k K8sInMemory) RolloutStatus(kind string, name string, options *K8sOptions) error {
	_, err := k.GetObject(kind, name, options)
	if err != nil {
		return err
	}
	return nil
}

// Wait -
func (k K8sInMemory) Wait(kind string, name string, condition string, options *K8sOptions) error {
	return k.RolloutStatus(kind, name, options)
}

// DeleteObject -
func (k K8sInMemory) DeleteObject(kind string, name string, options *K8sOptions) error {
	delete(k.objects, k.key(kind, name, "", options))
	return nil
}

// Apply -
func (k K8sInMemory) Apply(output ObjectStream, options *K8sOptions) error {
	return output(func(obj *Object) error {
		k.objects[k.key(obj.Kind, obj.MetaData.Name, obj.MetaData.Namespace, options)] = *obj
		return nil
	})
}

// Delete -
func (k K8sInMemory) Delete(output ObjectStream, options *K8sOptions) error {
	return output(func(obj *Object) error {
		delete(k.objects, k.key(obj.Kind, obj.MetaData.Name, obj.MetaData.Namespace, options))
		return nil
	})
}

// Get -
func (k K8sInMemory) Get(kind string, name string, options *K8sOptions) (*Object, error) {
	return k.GetObject(kind, name, options)
}

// Patch -
func (k K8sInMemory) Patch(kind string, name string, pt types.PatchType, patchJSON string, options *K8sOptions) (*Object, error) {
	obj, err := k.GetObject(kind, name, options)
	if err != nil {
		return nil, err
	}
	if pt != types.JSONPatchType {
		return nil, errors.New("Not implemented")
	}
	patch, err := jsonpatch.DecodePatch([]byte(patchJSON))
	if err != nil {
		return nil, err
	}
	original, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	modified, err := patch.Apply(original)
	if err != nil {
		return nil, err
	}
	modifyedObj := &Object{}
	err = json.Unmarshal(modified, modifyedObj)
	if err != nil {
		return nil, err
	}
	k.objects[k.key(obj.Kind, obj.MetaData.Name, obj.MetaData.Namespace, options)] = *modifyedObj
	return modifyedObj, nil
}

// List -
func (k K8sInMemory) List(kind string, options *K8sOptions, listOptions *ListOptions) (*Object, error) {
	return nil, errors.New("Not implemented")
}

// IsNotExist -
func (k K8sInMemory) IsNotExist(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

// ConfigContent -
func (k K8sInMemory) ConfigContent() *string {
	return nil
}

// ForConfig -
func (k K8sInMemory) ForConfig(config string) (K8s, error) {
	return k, nil
}

func (k K8sInMemory) key(kind, name, namespace string, options *K8sOptions) string {
	kind = strings.ToLower(kind)
	if isNameSpaced(kind) {
		if len(namespace) != 0 {
			return fmt.Sprintf("%s/%s/%s", namespace, kind, name)
		}
		if options != nil && options.Namespace != "" {
			return fmt.Sprintf("%s/%s/%s", options.Namespace, kind, name)
		}
		return fmt.Sprintf("%s/%s/%s", k.namespace, kind, name)
	}
	return fmt.Sprintf("%s/%s", kind, name)
}

// GetObject -
func (k K8sInMemory) GetObject(kind string, name string, options *K8sOptions) (*Object, error) {
	obj, ok := k.objects[k.key(kind, name, "", options)]
	if !ok {
		keys := []string{}
		for k := range k.objects {
			keys = append(keys, k)
		}
		if options != nil && options.IgnoreNotFound {
			return nil, nil
		}
		return nil, notFoundError(fmt.Sprintf("NotFound: %s %s ", k.key(kind, name, "", options), strings.Join(keys, ", ")))
	}
	return &obj, nil
}

func (k K8sInMemory) CreateOrUpdate(obj *Object, mutate func(obj *Object) error, options *K8sOptions) (*Object, error) {
	old, err := k.GetObject(obj.Kind, obj.MetaData.Name, options)
	if err != nil {
		if !k.IsNotExist(err) {
			return nil, err
		}
	} else {
		obj = old
	}
	err = mutate(obj)
	if err != nil {
		return nil, err
	}
	k.objects[k.key(obj.Kind, obj.MetaData.Name, obj.MetaData.Namespace, options)] = *obj
	return obj, nil
}

func (k K8sInMemory) DeleteByName(kind string, name string, options *K8sOptions) error {
	delete(k.objects, k.key(kind, name, "", options))
	return nil
}

// Progress -
func (k K8sInMemory) Progress(progress int) {
}
