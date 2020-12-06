package shalm

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	. "github.com/wonderix/shalm/pkg/shalm/test"
)

var _ = Describe("k8s", func() {

	Context("K8sConfigs", func() {
		It("args are correct", func() {
			args := K8sConfigs{}
			flagsSet := pflag.FlagSet{}
			args.AddFlags(&flagsSet)
			Expect(flagsSet.FlagUsages()).To(ContainSubstring(`-t, --tool tool`))
			Expect(flagsSet.FlagUsages()).To(ContainSubstring(`Tool to do the installation. Possible values kubectl (default) and kapp (default kubectl)`))
		})
	})

	Context("Tool", func() {
		It("set works correct", func() {
			var p Tool
			Expect(p.Set("kapp")).NotTo(HaveOccurred())
			Expect(p).To(BeEquivalentTo(ToolKapp))
			Expect(p.Set("kubectl")).NotTo(HaveOccurred())
			Expect(p).To(BeEquivalentTo(ToolKubectl))
			Expect(p.Set("")).NotTo(HaveOccurred())
			Expect(p).To(BeEquivalentTo(ToolKubectl))
			Expect(p.Set("invalid")).To(HaveOccurred())
		})

		It("string returns correct values", func() {
			Expect(Tool(ToolKapp).String()).To(Equal("kapp"))
			Expect(Tool(ToolKubectl).String()).To(Equal("kubectl"))
		})

	})

	Context("kapp", func() {
		var cmdArgs []string
		k8s := k8sImpl{command: func(_ context.Context, name string, arg ...string) *exec.Cmd {
			cmdArgs = arg
			return exec.Command("echo", `{ "kind" : "Deployment" }`)
		}, K8sConfigs: K8sConfigs{tool: ToolKapp, kubeConfig: "test"}, ctx: context.Background()}
		It("delete works", func() {
			err := k8s.Delete(func(writer ObjectWriter) error { return nil }, &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("apply works", func() {
			err := k8s.Apply(func(writer ObjectWriter) error { return nil }, &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("namespace is set to default if kubeconfig is given", func() {
			err := k8s.Apply(func(writer ObjectWriter) error { return nil }, &K8sOptions{ClusterScoped: true})
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdArgs).To(ContainElements("-n", "default"))
		})
	})
	Context("kubectl", func() {

		progress := 0
		k8s := k8sImpl{command: func(_ context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("echo", `{ "kind" : "Deployment" }`)
		}, app: "app", version: semver.MustParse("1.2"), namespace: "namespace",
			ctx: context.Background(),
			K8sConfigs: K8sConfigs{
				progressSubscription: func(p int) {
					progress = p
				},
				kubeConfig: "/tmp/test",
			}}
		k2 := k8s.ForSubChart("ns", "app", &semver.Version{}, 0)

		It("kubeconfig is copied", func() {
			Expect(k8s.kubeConfig).To(Equal(k2.(*k8sImpl).kubeConfig))
		})
		It("apply works", func() {
			err := k8s.Apply(func(writer ObjectWriter) error { return nil }, &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("delete works", func() {
			err := k8s.Delete(func(writer ObjectWriter) error { return nil }, &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("delete object works", func() {
			err := k8s.DeleteObject("kind", "name", &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("rollout status works", func() {
			err := k8s.RolloutStatus("kind", "name", &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("for namespace works", func() {
			Expect(k2.(*k8sImpl).namespace).To(Equal("ns"))
		})
		It("get works", func() {
			obj, err := k8s.Get("kind", "name", &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Kind).To(Equal("Deployment"))
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
		It("progress subscription works", func() {
			err := k2.Apply(func(writer ObjectWriter) error { k2.(*k8sImpl).progressCb(1, 1); return nil }, &K8sOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(progress).To(Equal(90))
		})
		It("Adds labels", func() {
			obj := k8s.objMapper()(&Object{})
			Expect(obj.MetaData.Labels).To(HaveKeyWithValue("shalm.wonderix.github.com/app", "app"))
			Expect(obj.MetaData.Labels).To(HaveKeyWithValue("shalm.wonderix.github.com/version", "1.2.0"))
			Expect(obj.MetaData.Namespace).To(Equal("namespace"))
		})
	})

	It("sorts by kind", func() {
		var err error
		s := func(writer ObjectWriter) error {
			writer(&Object{Kind: "Other"})
			writer(&Object{Kind: "StatefulSet"})
			writer(&Object{Kind: "Service"})
			return nil
		}

		By("Sorts in install order")
		writer := &bytes.Buffer{}
		err = prepare(s, false, func(obj *Object) *Object { return obj })(writer)
		Expect(err).ToNot(HaveOccurred())
		Expect(writer.String()).To(Equal(`
---
{"kind":"Service"}

---
{"kind":"StatefulSet"}

---
{"kind":"Other"}
`))

		By("Sorts in uninstall order")
		writer = &bytes.Buffer{}
		err = prepare(s, true, func(obj *Object) *Object { return obj })(writer)
		Expect(err).ToNot(HaveOccurred())
		Expect(writer.String()).To(Equal(`
---
{"kind":"Other"}

---
{"kind":"StatefulSet"}

---
{"kind":"Service"}
`))
	})
})
