package shalm

import (
	"encoding/json"
	"io"
	"time"

	"github.com/k14s/starlark-go/starlark"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("K8sValue", func() {

	It("behaves like starlark value", func() {
		var tool Tool
		k8s := &k8sValueImpl{&FakeK8s{
			InspectStub: func() string {
				return "kubeconfig = "
			},
			HostStub: func() string {
				return "test.local"
			},
			ToolStub: func() Tool {
				return tool
			},
			SetToolStub: func(t Tool) {
				tool = t
			},
		}}
		Expect(k8s.String()).To(ContainSubstring("kubeconfig = "))
		Expect(k8s.Type()).To(Equal("k8s"))
		Expect(func() { k8s.Hash() }).Should(Panic())
		Expect(k8s.Truth()).To(BeEquivalentTo(true))
		for _, method := range []string{"rollout_status", "delete", "get"} {
			value, err := k8s.Attr(method)
			Expect(err).NotTo(HaveOccurred())
			_, ok := value.(starlark.Callable)
			Expect(ok).To(BeTrue())
		}
		host, err := k8s.Attr("host")
		Expect(err).NotTo(HaveOccurred())
		Expect(host).To(BeEquivalentTo("test.local"))
		t, err := k8s.Attr("tool")
		Expect(err).NotTo(HaveOccurred())
		Expect(t).To(BeEquivalentTo(tool.String()))

		err = k8s.SetField("tool", starlark.String("kapp"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tool).To(BeEquivalentTo(ToolKapp))
		Expect(k8s.SetField("tool", starlark.String("xxx"))).To(HaveOccurred())
		Expect(k8s.SetField("xxx", starlark.String("xxx"))).To(HaveOccurred())
		Expect(k8s.AttrNames()).To(ConsistOf("rollout_status", "delete", "get", "wait", "for_config", "host", "tool"))
	})

	It("methods behave well", func() {
		fake := &FakeK8s{
			GetStub: func(kind string, name string, k8s *K8sOptions) (*Object, error) {
				return &Object{}, nil
			},
			ListStub: func(kind string, k8s *K8sOptions, listOptions *ListOptions) (*Object, error) {
				return &Object{}, nil
			},
			WatchStub: func(kind string, name string, k8s *K8sOptions) ObjectStream {
				return func(w ObjectWriter) error {
					return nil
				}
			},
		}
		k8s := &k8sValueImpl{fake}
		thread := &starlark.Thread{}
		for _, method := range []string{"rollout_status", "delete", "get"} {
			value, err := k8s.Attr(method)
			_, err = starlark.Call(thread, value, starlark.Tuple{starlark.String("kind"), starlark.String("object")},
				[]starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)},
					{starlark.String("namespaced"), starlark.Bool(true)}})
			Expect(err).NotTo(HaveOccurred())
		}
		{
			value, err := k8s.Attr("wait")
			_, err = starlark.Call(thread, value, starlark.Tuple{starlark.String("kind"), starlark.String("object"), starlark.String("condition")},
				[]starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)},
					{starlark.String("namespaced"), starlark.Bool(true)}})
			Expect(err).NotTo(HaveOccurred())
		}
		{
			value, err := k8s.Attr("list")
			_, err = starlark.Call(thread, value, starlark.Tuple{starlark.String("kind")}, []starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)},
				{starlark.String("namespaced"), starlark.Bool(true)}})
			Expect(err).NotTo(HaveOccurred())
		}
		{
			value, err := k8s.Attr("watch")
			_, err = starlark.Call(thread, value, starlark.Tuple{starlark.String("kind"), starlark.String("object")}, []starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)},
				{starlark.String("namespaced"), starlark.Bool(true)}})
			Expect(err).NotTo(HaveOccurred())
		}
		{
			value, err := k8s.Attr("progress")
			_, err = starlark.Call(thread, value, starlark.Tuple{starlark.MakeInt(0)}, nil)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(fake.RolloutStatusCallCount()).To(Equal(1))
		kind, name, options := fake.RolloutStatusArgsForCall(0)
		Expect(kind).To(Equal("kind"))
		Expect(name).To(Equal("object"))
		Expect(options.Timeout).To(Equal(10 * time.Second))
		Expect(options.ClusterScoped).To(BeFalse())
		Expect(fake.WaitCallCount()).To(Equal(1))
		Expect(fake.DeleteObjectCallCount()).To(Equal(1))
		Expect(fake.GetCallCount()).To(Equal(1))
	})

	It("watches objects", func() {
		fake := &FakeK8s{
			WatchStub: func(kind string, name string, options *K8sOptions) ObjectStream {
				return func(w ObjectWriter) error {
					obj := Object{Additional: map[string]json.RawMessage{"key": json.RawMessage([]byte(`"value"`))}}
					return w(&obj)
				}
			},
		}
		k8s := &k8sValueImpl{fake}
		thread := &starlark.Thread{}
		watch, err := k8s.Attr("watch")
		value, err := starlark.Call(thread, watch, starlark.Tuple{starlark.String("kind"), starlark.String("object")},
			[]starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)},
				{starlark.String("namespaced"), starlark.Bool(true)}})

		Expect(err).NotTo(HaveOccurred())
		iterable := value.(starlark.Iterable)
		iterator := iterable.Iterate()
		var obj starlark.Value
		found := iterator.Next(&obj)
		Expect(found).To(BeTrue())
		Expect(fake.WatchCallCount()).To(Equal(1))
		dict := UnwrapDict(obj).(*starlark.Dict)
		val, found, err := dict.Get(starlark.String("key"))
		Expect(found).To(BeTrue())
		Expect(val).To(Equal(starlark.String("value")))
	})

	It("applies objects", func() {
		var appliedObject Object
		fake := &FakeK8s{
			ApplyStub: func(s ObjectStream, options *K8sOptions) error {
				return s(func(o *Object) error {
					appliedObject = *o
					return nil
				})
			},
		}
		k8s := &k8sValueImpl{fake}
		thread := &starlark.Thread{}
		watch, err := k8s.Attr("apply")
		o := Object{MetaData: MetaData{
			Name: "test",
		}}
		_, err = starlark.Call(thread, watch, starlark.Tuple{ToStarlark(o)}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(appliedObject.MetaData.Name).To(Equal(o.MetaData.Name))
	})

	It("applies stream", func() {
		var appliedObject Object
		fake := &FakeK8s{
			ApplyStub: func(s ObjectStream, options *K8sOptions) error {
				return s(func(o *Object) error {
					appliedObject = *o
					return nil
				})
			},
		}
		k8s := &k8sValueImpl{fake}
		thread := &starlark.Thread{}
		watch, err := k8s.Attr("apply")
		o := Object{MetaData: MetaData{
			Name: "test",
		}}
		_, err = starlark.Call(thread, watch, starlark.Tuple{&stream{Stream: func(w io.Writer) error {
			_, err := w.Write([]byte("metadata:\n  name: test\n"))
			return err
		}}}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(appliedObject.MetaData.Name).To(Equal(o.MetaData.Name))
	})

})
