package cmd

import (
	"bytes"
	"path"
	"path/filepath"
	"runtime"

	semver "github.com/Masterminds/semver/v3"
	"github.com/wonderix/shalm/pkg/shalm"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o ./fake_k8s_test.go ../pkg/shalm K8s

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	root       = path.Join(filepath.Dir(b), "..")
	example    = path.Join(root, "charts", "example", "simple")
)

var _ = Describe("Apply Chart", func() {

	It("produces the correct output", func() {
		writer := bytes.Buffer{}
		k := &FakeK8s{
			ApplyStub: func(i shalm.ObjectStream, options *shalm.K8sOptions) error {
				return i.Encode()(&writer)
			},
		}
		k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) shalm.K8s {
			return k
		}
		k.GetStub = func(s string, s2 string, options *shalm.K8sOptions) (*shalm.Object, error) {
			return &shalm.Object{}, nil
		}

		err := apply(path.Join(example, "cf"), k, shalm.WithNamespace("mynamespace"))
		Expect(err).ToNot(HaveOccurred())
		output := writer.String()
		Expect(output).To(ContainSubstring("CREATE OR REPLACE USER 'uaa'"))
		Expect(k.RolloutStatusCallCount()).To(Equal(1))
		Expect(k.ApplyCallCount()).To(Equal(3))
		Expect(k.ForSubChartCallCount()).To(Equal(3))
		namespace, _, _, _ := k.ForSubChartArgsForCall(0)
		Expect(namespace).To(Equal("mynamespace"))
		namespace, _, _, _ = k.ForSubChartArgsForCall(1)
		Expect(namespace).To(Equal("mynamespace"))
		namespace, _, _, _ = k.ForSubChartArgsForCall(2)
		Expect(namespace).To(Equal("uaa"))
		kind, name, _ := k.RolloutStatusArgsForCall(0)
		Expect(name).To(Equal("mariadb-master"))
		Expect(kind).To(Equal("statefulset"))
	})

	It("produces correct objects", func() {
		k := shalm.NewK8sInMemory("default")

		err := apply(path.Join(example, "cf"), k, shalm.WithNamespace("mynamespace"))
		Expect(err).ToNot(HaveOccurred())
		uaa := k.ForSubChart("uaa", "uaa", &semver.Version{}, 0).(*shalm.K8sInMemory)
		_, err = uaa.GetObject("secret", "uaa-secret", nil)
		Expect(err).ToNot(HaveOccurred())
		my := k.ForSubChart("mynamespace", "uaa", &semver.Version{}, 0).(*shalm.K8sInMemory)
		_, err = my.GetObject("statefulset", "mariadb-master", nil)
		Expect(err).ToNot(HaveOccurred())
	})

})
