package cmd

import (
	"bytes"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wonderix/shalm/pkg/k8s"
)

var _ = Describe("Template Chart", func() {

	It("template the correct output", func() {
		writer := &bytes.Buffer{}
		err := template(path.Join(example, "cf"), k8s.NewK8sInMemoryEmpty())(writer)
		Expect(err).ToNot(HaveOccurred())
		output := writer.String()
		Expect(output).To(ContainSubstring("CREATE OR REPLACE USER 'uaa'"))
	})
})
