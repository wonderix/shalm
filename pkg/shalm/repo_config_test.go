package shalm

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/wonderix/shalm/pkg/shalm/test"
)

var _ = Describe("Repo Config", func() {

	It("token works", func() {
		c := &repoConfigs{}
		err := WithTokenAuth("url", "token")(c)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Credentials).To(HaveLen(1))
		Expect(c.Credentials[0].URL).To(Equal("url"))
		Expect(c.Credentials[0].Token).To(Equal("token"))
	})

	It("basic auth works", func() {
		c := &repoConfigs{}
		err := WithBasicAuth("url", "username", "password")(c)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Credentials).To(HaveLen(1))
		Expect(c.Credentials[0].URL).To(Equal("url"))
		Expect(c.Credentials[0].Username).To(Equal("username"))
		Expect(c.Credentials[0].Password).To(Equal("password"))
	})

	It("catalog works", func() {
		c := &repoConfigs{}
		err := WithCatalog("url")(c)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Catalogs).To(HaveLen(1))
		Expect(c.Catalogs[0]).To(Equal("url"))
	})

	It("config file works", func() {
		c := &repoConfigs{}
		dir := NewTestDir()
		defer dir.Remove()
		dir.WriteFile("config.yaml", []byte(`
credentials:
  - url : url
    username: username
    password: password
`), 0644)

		err := WithConfigFile(dir.Join("config.yaml"))(c)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Credentials).To(HaveLen(1))
		Expect(c.Credentials[0].URL).To(Equal("url"))
		Expect(c.Credentials[0].Username).To(Equal("username"))
		Expect(c.Credentials[0].Password).To(Equal("password"))
	})

})
