package shalm

import (
	"github.com/k14s/starlark-go/starlark"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Property", func() {
	It("behaves like starlark value", func() {
		thread := &starlark.Thread{Name: "main"}
		kwargs := []starlark.Tuple{{starlark.String("default"), starlark.String("xxx")}}
		d, err := makeProperty(thread, nil, nil, kwargs)
		Expect(err).NotTo(HaveOccurred())
		s := d.(*property)
		Expect(s.String()).To(ContainSubstring("type = string"))
		Expect(s.Truth()).To(BeEquivalentTo(false))
		Expect(s.Type()).To(Equal("property"))
		_, err = s.Hash()
		Expect(err).To(HaveOccurred())
		value := s.GetValue()
		Expect(value).To(Equal(starlark.None))

		value = s.GetValueOrDefault()
		Expect(value).To(Equal(starlark.String("xxx")))

		value, err = s.Attr("unknown")
		Expect(err).To(HaveOccurred())

		s.SetValue(starlark.String("test"))

		value = s.GetValue()
		Expect(value).To(Equal(starlark.String("test")))

	})

})
