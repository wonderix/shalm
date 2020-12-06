package shalm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/k14s/starlark-go/starlark"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/wonderix/shalm/pkg/shalm/test"
)

var _ = Describe("Chart", func() {

	Context("initialize", func() {

		It("reads Chart.yaml", func() {
			thread := &starlark.Thread{Name: "main"}
			dir := NewTestDir()
			defer dir.Remove()
			repo, _ := NewRepo()
			dir.WriteFile("Chart.yaml", []byte("name: mariadb\nversion: 6.12.2\n"), 0644)
			c, err := newChart(thread, repo, dir.Root())
			Expect(err).NotTo(HaveOccurred())
			Expect(c.GetName()).To(Equal("mariadb"))
		})
		It("reads Chart.yaml 'v' prefix in version", func() {
			thread := &starlark.Thread{Name: "main"}
			dir := NewTestDir()
			defer dir.Remove()
			repo, _ := NewRepo()
			dir.WriteFile("Chart.yaml", []byte("name: mariadb\nversion: v6.12.2\n"), 0644)
			c, err := newChart(thread, repo, dir.Root())
			Expect(err).NotTo(HaveOccurred())
			Expect(c.GetName()).To(Equal("mariadb"))
			Expect(c.GetVersion()).To(Equal(semver.MustParse("v6.12.2")))
		})

		It("reads values.yaml", func() {
			thread := &starlark.Thread{Name: "main"}
			dir := NewTestDir()
			defer dir.Remove()
			repo, _ := NewRepo()
			dir.WriteFile("values.yaml", []byte("replicas: \"1\"\ntimeout: \"30s\"\n"), 0644)
			c, err := newChart(thread, repo, dir.Root())
			Expect(err).NotTo(HaveOccurred())
			attr, err := c.Attr("replicas")
			Expect(err).NotTo(HaveOccurred())
			Expect(attr.(starlark.String).GoString()).To(Equal("1"))
			attr, err = c.Attr("timeout")
			Expect(err).NotTo(HaveOccurred())
			Expect(attr.(starlark.String).GoString()).To(Equal("30s"))
		})

		It("passes values from values.yaml to subchart", func() {
			thread := &starlark.Thread{Name: "main"}
			dir := NewTestDir()
			defer dir.Remove()
			repo, _ := NewRepo()
			dir.WriteFile("values.yaml", []byte("subchart: \n  timeout: \"30s\"\n"), 0644)
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	self.subchart = chart("subchart")
	self.subchart.key2 = "test"
`), 0644)
			dir.MkdirAll("subchart", 0755)
			dir.WriteFile("subchart/values.yaml", []byte("delay: \"30s\"\ntimeout: \"0s\"\n"), 0644)
			c, err := newChart(thread, repo, dir.Root())
			Expect(err).NotTo(HaveOccurred())
			attr, err := c.Attr("subchart")
			Expect(err).NotTo(HaveOccurred())
			subchart := attr.(ChartValue)
			attr, err = subchart.Attr("timeout")
			Expect(err).NotTo(HaveOccurred())
			Expect(attr.(starlark.String).GoString()).To(Equal("30s"))
			attr, err = subchart.Attr("key2")
			Expect(err).NotTo(HaveOccurred())
			Expect(attr.(starlark.String).GoString()).To(Equal("test"))
		})

	})
	Context("Chart.start", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		k8s := NewK8sInMemory("test")
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.WriteFile("values.yaml", []byte("timeout: \"30s\"\n"), 0644)
			dir.WriteFile("values_2.yaml", []byte("backup: true\n"), 0644)
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	self.timeout = "60s"
	self.load_yaml("values_2.yaml")
def method(self):
	return self.namespace
def apply(self,k8s):
	k8s.for_config('test')
	return self.__apply(k8s)
def template(self, glob = "",k8s = None):
	return '{ "Kind" : "hello" }'
def delete(self,k8s):
	return self.__delete(k8s)
`),
				0644)
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())
			err = c.Apply(thread, k8s)
			Expect(err).NotTo(HaveOccurred())

		})
		AfterEach(func() {
			dir.Remove()
		})
		It("evalutes constructor", func() {
			{
				attr, err := c.Attr("timeout")
				Expect(err).NotTo(HaveOccurred())
				Expect(attr.(starlark.String).GoString()).To(Equal("60s"))
			}
			{
				attr, err := c.Attr("backup")
				Expect(err).NotTo(HaveOccurred())
				Expect(bool(attr.(starlark.Bool))).To(Equal(true))
			}
		})
		It("binds method to self", func() {
			attr, err := c.Attr("method")
			Expect(err).NotTo(HaveOccurred())
			value, err := starlark.Call(thread, attr.(starlark.Callable), nil, nil)
			Expect(value.(starlark.String).GoString()).To(Equal("default"))
		})
		It("overrides apply", func() {
			attr, err := c.Attr("apply")
			Expect(err).NotTo(HaveOccurred())
			k := &FakeK8s{}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			_, err = starlark.Call(thread, attr.(starlark.Callable), starlark.Tuple{NewK8sValue(k)}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(k.ApplyCallCount()).To(Equal(1))
		})
		It("overrides delete", func() {
			attr, err := c.Attr("delete")
			Expect(err).NotTo(HaveOccurred())
			k := &FakeK8s{}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			_, err = starlark.Call(thread, attr.(starlark.Callable), starlark.Tuple{NewK8sValue(k)}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(k.DeleteCallCount()).To(Equal(1))
		})
		It("overrides template", func() {
			attr, err := c.Attr("template")
			Expect(err).NotTo(HaveOccurred())
			s := toStream(starlark.Call(thread, attr.(starlark.Callable), nil, nil))
			buf := &bytes.Buffer{}
			err = s(buf)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println(buf.String())
			Expect(buf.String()).To(Equal("{ \"Kind\" : \"hello\" }"))
		})
	})
	Context("Render embedded ytt template", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.MkdirAll("ytt", 0755)
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	self.timeout = "60s"
def template(self,glob=''):
	return self.ytt(inject("ytt/test.yml",self=self))
`),
				0644)
			dir.WriteFile("ytt/test.yml", []byte("#@ if True:\ntest: #@ self.timeout\n#@ end\n"), 0644)
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			dir.Remove()
		})
		It("applies ytt", func() {
			writer := bytes.Buffer{}
			k := &FakeK8s{
				ApplyStub: func(i ObjectStream, options *K8sOptions) error {
					return i.Encode()(&writer)
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err := c.Apply(thread, k)
			Expect(err).NotTo(HaveOccurred())
			Expect(k.ApplyCallCount()).To(Equal(1))
			Expect(writer.String()).To(Equal("\n---\n{\"test\":\"60s\"}\n"))
		})
	})
	Context("Render ytt template", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.MkdirAll("templates", 0755)
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	self.timeout = "60s"
def template(self,glob=''):
	return self.ytt(self.helm("templates"))
`),
				0644)
			dir.WriteFile("templates/test.yaml", []byte("#@ timeout = {{ .Values.timeout | quote }}\ntest: #@ timeout\n"), 0644)
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())

		})
		AfterEach(func() {
			dir.Remove()
		})
		It("applies ytt", func() {
			_, err := os.Stat("/usr/local/bin/ytt")
			if err != nil {
				Skip("ytt is not installed")
			}
			writer := bytes.Buffer{}
			k := &FakeK8s{
				ApplyStub: func(i ObjectStream, options *K8sOptions) error {
					return i.Encode()(&writer)
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err = c.Apply(thread, k)
			Expect(err).NotTo(HaveOccurred())
			Expect(k.ApplyCallCount()).To(Equal(1))
			Expect(writer.String()).To(Equal("\n---\n{\"test\":\"60s\"}\n"))
		})
	})
	Context("Render template in ytt-templates", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.MkdirAll("ytt-templates", 0755)
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	self.timeout = "60s"
`),
				0644)
			dir.WriteFile("ytt-templates/test.yaml", []byte("test: #@ self.timeout\n"), 0644)
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())

		})
		AfterEach(func() {
			dir.Remove()
		})
		It("applies ytt", func() {
			writer := bytes.Buffer{}
			k := &FakeK8s{
				ApplyStub: func(i ObjectStream, options *K8sOptions) error {
					return i.Encode()(&writer)
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err := c.Apply(thread, k)
			Expect(err).NotTo(HaveOccurred())
			Expect(k.ApplyCallCount()).To(Equal(1))
			Expect(writer.String()).To(Equal("\n---\n{\"test\":\"60s\"}\n"))
		})
	})
	Context("methods", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}

		BeforeEach(func() {
			dir = NewTestDir()
			dir.MkdirAll("templates", 0755)
			dir.WriteFile("templates/deployment.yaml", []byte("namespace: {{ .Release.Namespace}}"), 0644)
			dir.WriteFile("Chart.yaml", []byte("name: mariadb\nversion: 6.12.2\n"), 0644)
			repo, _ := NewRepo()
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithNamespace("namespace"), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())
			Expect(c.GetName()).To(Equal("mariadb"))

		})
		AfterEach(func() {
			dir.Remove()
		})
		It("templates a chart", func() {
			defer dir.Remove()
			Expect(c.GetName()).To(Equal("mariadb"))
			buf := &bytes.Buffer{}
			err := c.Template(thread, NewK8sInMemoryEmpty())(buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(Equal("---\nnamespace: namespace\n"))
		})

		It("applies a chart", func() {
			Expect(c.GetName()).To(Equal("mariadb"))
			writer := bytes.Buffer{}
			k := &FakeK8s{
				ApplyStub: func(i ObjectStream, options *K8sOptions) error {
					return i.Encode()(&writer)
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err := c.Apply(thread, k)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.String()).To(Equal("\n---\n{\"namespace\":\"namespace\"}\n"))
		})

		It("deletes a chart", func() {
			Expect(c.GetName()).To(Equal("mariadb"))
			writer := bytes.Buffer{}
			k := &FakeK8s{
				DeleteStub: func(i ObjectStream, options *K8sOptions) error {
					i.Encode()(&writer)
					return nil
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err := c.Delete(thread, k, &DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.String()).To(Equal("\n---\n{\"namespace\":\"namespace\"}\n"))
		})

		It("packages a chart", func() {
			writer := &bytes.Buffer{}
			err := c.Package(writer, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes.HasPrefix(writer.Bytes(), []byte{0x1F, 0x8B, 0x08})).To(BeTrue())
		})

		It("packages a helm chart", func() {
			writer := &bytes.Buffer{}
			err := c.Package(writer, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes.HasPrefix(writer.Bytes(), []byte{0x1F, 0x8B, 0x08})).To(BeTrue())
		})

		It("applies subcharts", func() {
			thread := &starlark.Thread{Name: "main"}
			dir := NewTestDir()
			defer dir.Remove()
			repo, _ := NewRepo()
			dir.MkdirAll("chart1/templates", 0755)
			dir.MkdirAll("chart2/templates", 0755)
			dir.WriteFile("chart1/Chart.star", []byte("def init(self):\n  self.chart2 = chart(\"../chart2\",namespace=\"chart2\")\n"), 0644)

			dir.WriteFile("chart2/templates/deployment.yaml", []byte("namespace: {{ .Release.Namespace}}"), 0644)
			dir.WriteFile("chart2/Chart.yaml", []byte("name: test\nversion: 1.0.0\n"), 0644)
			c, err := newChart(thread, repo, dir.Join("chart1"), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())
			writer := bytes.Buffer{}
			k := &FakeK8s{
				DeleteStub: func(i ObjectStream, options *K8sOptions) error {
					i.Encode()(&writer)
					return nil
				},
			}
			k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
				return k
			}
			err = c.Delete(thread, k, &DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(k.DeleteCallCount()).To(Equal(2))
			Expect(writer.String()).To(Equal("\n---\n{\"namespace\":\"chart2\"}\n"))
		})

	})
	Context("shalmignore", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}

		BeforeEach(func() {
			dir = NewTestDir()
			dir.MkdirAll("templates", 0755)
			dir.WriteFile("templates/deployment.yaml", []byte("namespace: {{ .Release.namespace}}"), 0644)
			dir.MkdirAll(".ignored", 0755)
			for i := 0; i < 100; i++ {
				dir.WriteFile(fmt.Sprintf(".ignored/test%d.md", i), []byte("0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"), 0644)
			}
			dir.WriteFile(".shalmignore", []byte(".ignored\n"), 0644)
			dir.WriteFile("Chart.yaml", []byte("name: mariadb\nversion: 6.12.2\n"), 0644)
			repo, _ := NewRepo()
			var err error
			c, err = newChart(thread, repo, dir.Root(), WithNamespace("namespace"), WithSkipChart(true))
			Expect(err).NotTo(HaveOccurred())
			Expect(c.GetName()).To(Equal("mariadb"))

		})
		AfterEach(func() {
			dir.Remove()
		})
		It("packages a chart", func() {
			writer := &bytes.Buffer{}
			err := c.Package(writer, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.Bytes()).To(HaveLen(198))
		})
	})
	It("behaves like starlark value", func() {
		thread := &starlark.Thread{Name: "main"}
		dir := NewTestDir()
		defer dir.Remove()
		repo, _ := NewRepo()
		dir.WriteFile("values.yaml", []byte("replicas: \"1\"\ntimeout: \"30s\"\n"), 0644)
		c, err := newChart(thread, repo, dir.Root())
		Expect(err).NotTo(HaveOccurred())
		Expect(c.String()).To(ContainSubstring("default = \"1\""))
		_, err = c.Hash()
		Expect(err).To(HaveOccurred())
		Expect(c.Truth()).To(BeEquivalentTo(true))
		Expect(c.Type()).To(Equal("chart"))
		value, err := c.Attr("name")
		Expect(err).NotTo(HaveOccurred())
		Expect(value.(starlark.String).GoString()).To(ContainSubstring("shalm"))
		value, err = c.Attr("namespace")
		Expect(err).NotTo(HaveOccurred())
		Expect(value.(starlark.String).GoString()).To(Equal("default"))
		value, err = c.Attr("apply")
		Expect(err).NotTo(HaveOccurred())
		Expect(value.(starlark.Callable).Name()).To(ContainSubstring("wrap_namespace at "))
		value, err = c.Attr("unknown")
		Expect(err).To(HaveOccurred())

		value, found, err := c.Get(starlark.String("replicas"))
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(value).To(Equal(starlark.String("1")))

		err = c.SetKey(starlark.String("timeout"), starlark.String("60s"))
		Expect(err).NotTo(HaveOccurred())

		value, found, err = c.Get(starlark.String("timeout"))
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(value).To(Equal(starlark.String("60s")))

		Expect(c.AttrNames()).To(ContainElement("template"))
	})

	It("applies a credentials ", func() {
		thread := &starlark.Thread{Name: "main"}
		dir := NewTestDir()
		defer dir.Remove()
		repo, _ := NewRepo()
		dir.WriteFile("Chart.star", []byte("def init(self):\n  self.cred = user_credential(\"test\")\n"), 0644)
		c, err := newChart(thread, repo, dir.Root(), WithSkipChart(true))
		Expect(err).NotTo(HaveOccurred())
		var obj Object
		k := &FakeK8s{
			ApplyStub: func(i ObjectStream, options *K8sOptions) error {
				return i(func(o *Object) error { obj = *o; return nil })
			},
			GetStub: func(kind string, name string, k8s *K8sOptions) (*Object, error) {
				return nil, errors.New("NotFound")
			},
			IsNotExistStub: func(err error) bool {
				return true
			},
		}
		k.ForSubChartStub = func(s string, app string, version *semver.Version, children int) K8s {
			return k
		}
		err = c.Apply(thread, k)
		Expect(err).NotTo(HaveOccurred())
		Expect(obj.APIVersion).To(Equal("v1"))
		Expect(obj.Kind).To(Equal("Secret"))
		Expect(obj.MetaData.Name).To(Equal("test"))
		var user struct {
			Username []byte `json:"username"`
			Password []byte `json:"password"`
		}
		json.Unmarshal(obj.Additional["data"], &user)
		Expect(user.Username).To(HaveLen(16))
		Expect(user.Password).To(HaveLen(16))
	})

	It("merges values ", func() {
		thread := &starlark.Thread{Name: "main"}
		dir := NewTestDir()
		defer dir.Remove()
		repo, _ := NewRepo()
		dir.WriteFile("Chart.star", []byte("def init(self):\n  self.timeout=50\n"), 0644)
		c, err := newChart(thread, repo, dir.Root())
		Expect(err).NotTo(HaveOccurred())
		c.mergeValues(map[string]interface{}{"timeout": 60, "string": "test"})
		Expect(c.values["timeout"]).To(Equal(starlark.MakeInt(60)))
		Expect(c.values["string"]).To(Equal(starlark.String("test")))
	})

	Context("Exchange templates between charts", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.MkdirAll("chart1/ytt", 0755)
			dir.WriteFile("chart1/ytt/values.yaml", []byte("#@ load(\"@ytt:data\", \"data\")\nconfig: #@ data.values.sys_domain\n"), 0644)
			dir.WriteFile("chart1/Chart.star", []byte(`
def init(self):
	self.config = property()
def template(self,glob=""):
   return self.ytt("ytt",self.config)
`), 0644)
			dir.MkdirAll("chart2/config", 0755)
			dir.WriteFile("chart2/config/values.yaml", []byte("#@data/values\n---\nsys_domain: {{ .Values.domain }}\n"), 0644)
			dir.WriteFile("chart2/Chart.star", []byte(`
def init(self):
    self.domain = "example.com"
    self.chart1 = chart("../chart1")
    self.chart1.config = self.helm("config")

`), 0644)
			var err error
			c, err = newChart(thread, repo, dir.Join("chart2"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("templates correct", func() {
			s := c.Template(thread, NewK8sInMemoryEmpty())
			out := &bytes.Buffer{}
			err := s(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.String()).To(ContainSubstring("config: example.com"))
		})
		AfterEach(func() {
			dir.Remove()
		})

	})
	Context("Referencing charts", func() {
		var dir TestDir
		var c ChartValue
		thread := &starlark.Thread{Name: "main"}
		BeforeEach(func() {
			dir = NewTestDir()
			repo, _ := NewRepo()
			dir.WriteFile("Chart.star", []byte(`
def init(self):
	pass
`), 0644)
			var err error
			c, err = newChart(thread, repo, dir.Root())
			Expect(err).NotTo(HaveOccurred())
		})
		It("counts references correctly", func() {
			k := NewK8sInMemoryEmpty()
			err := c.Apply(thread, k)
			Expect(err).NotTo(HaveOccurred())
			By("add reference", func() {
				references, err := c.AddUsedBy("test", k)
				Expect(err).NotTo(HaveOccurred())
				Expect(references).To(Equal(1))
			})
			By("remove reference", func() {
				references, err := c.RemoveUsedBy("test", k)
				Expect(err).NotTo(HaveOccurred())
				Expect(references).To(Equal(0))
			})
		})
		AfterEach(func() {
			dir.Remove()
		})

	})

})
