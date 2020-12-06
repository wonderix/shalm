package shalm

import (
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"runtime"

	runtime2 "k8s.io/apimachinery/pkg/runtime"

	"github.com/Masterminds/semver/v3"
	"github.com/k14s/starlark-go/starlark"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	shalmv1a2 "github.com/wonderix/shalm/api/v1alpha2"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	root       = path.Join(filepath.Dir(b), "..", "..")
	example    = path.Join(root, "charts", "example", "simple")
)

var _ = Describe("Repo", func() {

	Context("push chart", func() {
		var repo Repo
		var thread *starlark.Thread

		BeforeEach(func() {
			thread = &starlark.Thread{Name: "main"}
			repo, _ = NewRepo()
		})
		It("reads chart from directory", func() {
			chart, err := repo.Get(thread, path.Join(example, "mariadb"), WithNamespace("namespace"))
			Expect(err).ToNot(HaveOccurred())
			Expect(chart.GetName()).To(Equal("mariadb"))
		})
		It("reads chart from tar file", func() {
			chart, err := repo.Get(thread, path.Join(example, "mariadb-6.12.2.tgz"), WithNamespace("namespace"))
			Expect(err).ToNot(HaveOccurred())
			Expect(chart.GetName()).To(Equal("mariadb"))
		})
		It("reads chart from zip file", func() {
			chart, err := repo.Get(thread, path.Join(example, "mariadb-6.12.2.zip"), WithNamespace("namespace"))
			Expect(err).ToNot(HaveOccurred())
			Expect(chart.GetName()).To(Equal("mariadb"))
		})
		It("reads chart from http", func() {

			http.HandleFunc("/mariadb.tgz", func(w http.ResponseWriter, r *http.Request) {
				content, _ := ioutil.ReadFile(path.Join(example, "mariadb-6.12.2.tgz"))
				w.Write(content)
			})

			go http.ListenAndServe("127.0.0.1:8675", nil)
			for {
				con, err := net.Dial("tcp", "127.0.0.1:8675")
				if err == nil {
					con.Close()
					break
				}
			}
			chart, err := repo.Get(thread, "http://localhost:8675/mariadb.tgz", WithNamespace("namespace"))
			Expect(err).ToNot(HaveOccurred())
			Expect(chart.GetName()).To(Equal("mariadb"))
		})
		It("creates chart from spec", func() {
			tgz, err := ioutil.ReadFile(path.Join(example, "mariadb-6.12.2.tgz"))
			Expect(err).ToNot(HaveOccurred())
			chart, err := repo.GetFromSpec(thread, &shalmv1a2.ChartSpec{
				Namespace: "namespace",
				ChartTgz:  tgz,
				Values:    runtime2.RawExtension{Raw: []byte(`{ "timeout" : 8 , "name" : "test"}`)},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(chart.GetName()).To(Equal("mariadb"))
			Expect(chart.(*chartImpl).values["timeout"]).To(Equal(starlark.Float(8))) // json.Unmarshal converts always to float64
			Expect(chart.(*chartImpl).values["name"]).To(Equal(starlark.String("test")))
		})

	})
	version := semver.MustParse("v0.6.1")
	Context("name and version", func() {
		It("guesses github releases correct", func() {
			options := NewGenusAndVersion("https://github.com/wonderix/shalm/releases/download/v0.6.1/shalm-0.6.1-dirty.tgz")
			Expect(options.genus).To(Equal("github.com_wonderix_shalm"))
			Expect(options.version).To(Equal(version))
		})
		It("guesses github archives correct", func() {
			options := NewGenusAndVersion("https://github.com/wonderix/shalm/archive/0.6.1.zip")
			Expect(options.genus).To(Equal("github.com_wonderix_shalm"))
			Expect(options.version).To(Equal(semver.MustParse("0.6.1")))
		})
		It("guesses github enterprise archives correct", func() {
			options := NewGenusAndVersion("https://github.tools.sap/api/v3/repos/cki/cf-for-k8s-scp/zipball/v0.6.1")
			Expect(options.genus).To(Equal("github.tools.sap_cki_cf-for-k8s-scp"))
			Expect(options.version).To(Equal(version))
		})
		It("other url matches", func() {
			options := NewGenusAndVersion("https://test.com/test/v0.6.1")
			Expect(options.genus).To(Equal("test.com_test"))
			Expect(options.version).To(Equal(version))
		})
		It("catalog matches", func() {
			options := NewGenusAndVersion("catalog:istio")
			Expect(options.genus).To(Equal("istio"))
		})

	})
})
