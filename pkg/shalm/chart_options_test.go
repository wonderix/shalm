package shalm

import (
	"fmt"
	"os"

	"github.com/k14s/starlark-go/starlark"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	. "github.com/wonderix/shalm/pkg/shalm/test"
)

var _ = Describe("Chart Options", func() {
	Context("Properties", func() {
		It("produces the correct output", func() {
			properties := Properties{}
			err := properties.Set("a=b=c")
			Expect(err).NotTo(HaveOccurred())
			Expect(properties.get("a")).To(Equal(starlark.String("b=c")))
		})

	})
	Context("propertiesYamlVar", func() {
		It("produces the correct output", func() {
			dir := NewTestDir()
			dir.WriteFile("test.yaml", []byte("a: b\nc: d\n"), 0644)
			defer dir.Remove()
			properties := Properties{}
			propertiesYaml := propertiesYamlVar{properties: &properties}
			err := propertiesYaml.Set(fmt.Sprintf("a=%s", dir.Join("test.yaml")))
			Expect(err).NotTo(HaveOccurred())
			c := properties.get("a").(*starlark.Dict)
			Expect(c.Len()).To(Equal(2))
		})

	})
	Context("proeprtiesFileVar", func() {
		It("produces the correct output", func() {
			dir := NewTestDir()
			dir.WriteFile("test.txt", []byte("hello"), 0644)
			defer dir.Remove()
			properties := Properties{}
			propertiesFile := proeprtiesFileVar{properties: &properties}
			err := propertiesFile.Set(fmt.Sprintf("a=%s", dir.Join("test.txt")))
			Expect(err).NotTo(HaveOccurred())
			Expect(properties.get("a")).To(Equal(starlark.String("hello")))
		})

	})
	Context("propertiesEnvVar", func() {
		It("produces the correct output", func() {
			properties := Properties{}
			propertiesEnv := propertiesEnvVar{properties: &properties}
			os.Setenv("SHALM_TEST", "XXXX")
			err := propertiesEnv.Set("test=SHALM_TEST")
			Expect(err).NotTo(HaveOccurred())
			Expect(properties.get("test")).To(Equal(starlark.String("XXXX")))
		})

	})
	Context("valuesFile", func() {
		It("produces the correct output", func() {
			dir := NewTestDir()
			dir.WriteFile("test.yaml", []byte("test: test\ntimeout: timeout"), 0644)
			defer dir.Remove()
			properties := Properties{}
			propertiesFile := propertiesFile{properties: &properties}
			err := propertiesFile.Set(dir.Join("test.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(properties.get("test")).To(Equal(starlark.String("test")))
			Expect(properties.get("timeout")).To(Equal(starlark.String("timeout")))
		})

	})
	Context("ChartOptions", func() {
		It("args are correct", func() {
			args := ChartOptions{}
			flagsSet := pflag.FlagSet{}
			args.AddFlags(&flagsSet)
			Expect(flagsSet.FlagUsages()).To(ContainSubstring(` --set properties             Set values (key=val)`))
			Expect(flagsSet.FlagUsages()).To(ContainSubstring(`-n, --namespace string           namespace for installation (default "default")`))
			Expect(flagsSet.FlagUsages()).To(ContainSubstring(`-s, --suffix string              Suffix which is used to build the chart name`))
		})
	})
})
