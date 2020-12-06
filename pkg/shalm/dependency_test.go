package shalm

import (
	"github.com/k14s/starlark-go/starlark"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dependency", func() {
	It("behaves like starlark value", func() {
		thread := &starlark.Thread{Name: "main"}
		args := starlark.Tuple{starlark.String("url"), starlark.String(">= 1.0")}
		d, err := makeDependency(nil, nil, "")(thread, nil, args, nil)
		Expect(err).NotTo(HaveOccurred())
		s := d.(*dependency)
		Expect(s.String()).To(ContainSubstring("url = url"))
		Expect(s.Truth()).To(BeEquivalentTo(true))
		Expect(s.Type()).To(Equal("dependency"))

		value, err := s.Attr("unknown")
		Expect(err).To(HaveOccurred())

		s.SetField("test", starlark.String("test"))
		value, err = s.Attr("test")
		Expect(err).NotTo(HaveOccurred())
		Expect(value.(starlark.String).GoString()).To(Equal("test"))

	})

})
