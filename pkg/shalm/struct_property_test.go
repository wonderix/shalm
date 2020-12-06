package shalm

import (
	"github.com/k14s/starlark-go/starlark"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StructProperty", func() {
	It("behaves like starlark value", func() {
		thread := &starlark.Thread{Name: "main"}
		p := newProperty(starlark.String("test"))
		kwargs := []starlark.Tuple{{starlark.String("property1"), p}}
		d, err := makeStructProperty(thread, nil, nil, kwargs)
		Expect(err).NotTo(HaveOccurred())
		s := d.(*structProperty)
		Expect(s.String()).To(ContainSubstring("properties = "))
		Expect(s.Truth()).To(BeEquivalentTo(true))
		_, err = s.Hash()
		Expect(err).To(HaveOccurred())
		Expect(s.Type()).To(Equal("struct_property"))

		Expect(s.GetValue().(*starlark.Dict).Len()).To(Equal(0))
		Expect(s.GetValueOrDefault().(*starlark.Dict).Len()).To(Equal(1))

		_, err = s.Attr("unknown")
		Expect(err).To(HaveOccurred())

		err = s.SetValue(starlark.String("test"))
		Expect(err).To(HaveOccurred())

		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("property1"), starlark.String("value"))
		err = s.SetValue(dict)
		Expect(err).NotTo(HaveOccurred())

		value, err := s.Attr("property1")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal(starlark.String("value")))

	})
	It("allows additional properties", func() {
		s := newStructProperty(true)
		err := s.SetField("test", starlark.String("test"))
		Expect(err).NotTo(HaveOccurred())
		d := starlark.NewDict(1)
		d.SetKey(starlark.String("dict"), starlark.String("dict"))
		err = s.SetField("dict", d)
		Expect(err).NotTo(HaveOccurred())
	})
})
