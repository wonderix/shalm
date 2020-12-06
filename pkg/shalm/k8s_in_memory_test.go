package shalm

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/wonderix/shalm/pkg/shalm/test"
)

var _ = Describe("k8s_in_memory", func() {
	var k8s *K8sInMemory
	namespace := "test"
	secret := Object{Kind: "Secret", MetaData: MetaData{Name: "test", Namespace: namespace, Annotations: map[string]string{"annotation": "annotation"}}}

	BeforeEach(func() {
		k8s = NewK8sInMemory(namespace)
	})

	It("apply works", func() {
		err := k8s.Apply(func(writer ObjectWriter) error {
			return writer(&Object{Kind: "Secret", MetaData: MetaData{Name: "test"}})
		}, &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
		obj, err := k8s.GetObject("secret", "test", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Kind).To(Equal("Secret"))
	})
	It("delete works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		err := k8s.Delete(func(writer ObjectWriter) error {
			return writer(&Object{Kind: "Secret", MetaData: MetaData{Name: "test"}})
		}, &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = k8s.GetObject("secret", "test", nil)
		Expect(k8s.IsNotExist(err)).To(BeTrue())
	})
	It("delete object works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		err := k8s.DeleteObject("secret", "test", &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
	It("rollout status works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		err := k8s.RolloutStatus("secret", "test", &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
	It("watch works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		stream := k8s.Watch("secret", "test", &K8sOptions{})
		var obj Object
		err := stream(func(o *Object) error { obj = *o; return nil })
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Kind).To(Equal("Secret"))
	})
	It("for namespace works", func() {
		k2 := k8s.ForSubChart("ns", "app", &semver.Version{}, 0)
		Expect(k2.(*K8sInMemory).namespace).To(Equal("ns"))
	})
	It("get works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		obj, err := k8s.Get("secret", "test", &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.Kind).To(Equal("Secret"))
	})
	It("patch works", func() {
		k8s = NewK8sInMemory(namespace, secret)
		obj, err := k8s.Patch("secret", "test", types.JSONPatchType, `[{"op": "add", "path": "/metadata/annotations/test", "value" : "xxx"}]`, &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.MetaData.Annotations["test"]).To(Equal("xxx"))
		obj, err = k8s.Patch("secret", "test", types.JSONPatchType, `[{"op": "remove", "path": "/metadata/annotations/test"}]`, &K8sOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.MetaData.Annotations).NotTo(HaveKey("test"))

	})
	It("ConfigContent works", func() {
		dir := NewTestDir()
		defer dir.Remove()
		dir.MkdirAll("chart2/templates", 0755)
		dir.WriteFile("kubeconfig", []byte("hello"), 0644)
		k8s := k8sImpl{K8sConfigs: K8sConfigs{kubeConfig: dir.Join("kubeconfig")}, ctx: context.Background()}
		content := k8s.ConfigContent()
		Expect(content).NotTo(BeNil())
		Expect(*content).To(Equal("hello"))
	})

})
