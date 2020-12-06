package shalm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type k8sClient struct {
	client *rest.RESTClient
}

type request struct {
	request   *rest.Request
	namespace *string
	resource  string
	name      string
}

type result struct {
	rest.Result
	err error
}

type errUnknownResource struct {
	resource string
}

func (e *errUnknownResource) Error() string {
	return fmt.Sprintf("Unknown resource %s", e.resource)
}

var kindToGroupVersionKind = map[string]schema.GroupVersionKind{}

func init() {
	kinds := []schema.GroupVersionKind{}
	for k := range scheme.Scheme.AllKnownTypes() {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Version > kinds[j].Version })
	for _, k := range kinds {
		groupVersion := schema.GroupVersionKind{Version: k.Version, Group: k.Group, Kind: strings.ToLower(k.Kind) + "s"}
		kindToGroupVersionKind[strings.ToLower(k.Kind)] = groupVersion
		kindToGroupVersionKind[strings.ToLower(k.Kind)+"."+groupVersion.Group] = groupVersion
		kindToGroupVersionKind[groupVersion.Kind] = groupVersion
		kindToGroupVersionKind[groupVersion.Kind+"."+groupVersion.Group] = groupVersion
	}
}

func configKube(kubeConfig string) (*rest.Config, error) {
	if len(kubeConfig) == 0 {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		if len(host) != 0 {
			return rest.InClusterConfig()
		}
		env, ok := os.LookupEnv("KUBECONFIG")
		if ok {
			kubeConfig = env
		} else {
			path := filepath.Join(homeDir(), ".kube", "config")
			kubeConfig = path
		}
	}
	return clientcmd.BuildConfigFromFlags("", kubeConfig)

}
func newK8sClient(config *rest.Config) (*k8sClient, error) {
	config.NegotiatedSerializer = scheme.Codecs
	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, err
	}
	scheme.Scheme.AllKnownTypes()
	return &k8sClient{client: client}, nil

}

func (k *k8sClient) Get() request {
	return request{request: k.client.Get()}
}

func (k *k8sClient) Patch(pt types.PatchType) request {
	return request{request: k.client.Patch(pt)}
}

func (k *k8sClient) Post() request {
	return request{request: k.client.Post()}
}

func (k *k8sClient) Put() request {
	return request{request: k.client.Put()}
}

func (k *k8sClient) Delete() request {
	return request{request: k.client.Delete()}
}

func (r request) Namespace(namespace *string) request {
	r.namespace = namespace
	return r
}

func (r request) Resource(resource string) request {
	r.resource = resource
	return r
}

func (r request) Name(name string) request {
	r.name = name
	return r
}

func (r request) Body(obj interface{}) request {
	r.request.Body(obj)
	return r
}

func (r request) Do() result {
	gv, ok := kindToGroupVersionKind[strings.ToLower(r.resource)]
	if !ok {
		return result{err: &errUnknownResource{resource: r.resource}}
	}

	prefix := ""
	if len(gv.Group) == 0 {
		prefix = "api"
	} else {
		prefix = "apis"
	}

	if r.namespace != nil {
		r.request.AbsPath(prefix, gv.Group, gv.Version, "namespaces", *r.namespace, gv.Kind, r.name)
	} else {
		r.request.AbsPath(prefix, gv.Group, gv.Version, gv.Kind, r.name)
	}
	return result{Result: r.request.Do()}
}

func (r result) Get() (*Object, error) {
	if r.err != nil {
		return nil, r.err
	}
	data, err := r.Result.Raw()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("0-length response")
	}
	var obj Object
	if err = json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj.Kind == "Status" {
		return nil, errors.New(string(data))
	}
	return &obj, nil
}

func (r result) Error() error {
	if r.err != nil {
		return r.err
	}
	return r.Result.Error()
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
