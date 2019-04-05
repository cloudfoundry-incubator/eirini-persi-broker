package config_test

import (
	"os"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	brokerconfig "github.com/suse/eirini-persi-broker/config"
)

var _ = Describe("parsing the broker config file", func() {

	Describe("ParseConfig", func() {
		var (
			config         brokerconfig.Config
			configPath     string
			parseConfigErr error
		)

		BeforeEach(func() {
			configPath = "test_config.yml"
		})

		JustBeforeEach(func() {
			path, err := filepath.Abs(path.Join("assets", configPath))
			Ω(err).ToNot(HaveOccurred())
			config, parseConfigErr = brokerconfig.ParseConfig(path)
		})

		Context("when the configuration is valid", func() {

			var dirs []string

			AfterEach(func() {
				for _, dir := range dirs {
					err := os.RemoveAll(dir)
					Ω(err).ShouldNot(HaveOccurred())
				}
			})

			It("does not error", func() {
				Ω(parseConfigErr).NotTo(HaveOccurred())
			})

			It("loads service name", func() {
				Ω(config.ServiceConfiguration.ServiceName).To(Equal("my-service"))
			})

			It("loads service id", func() {
				Ω(config.ServiceConfiguration.ServiceID).To(Equal("12345abcde"))
			})

			It("loads namespace", func() {
				Ω(config.Namespace).To(Equal("eirini"))
			})

			It("loads host", func() {
				Ω(config.Host).To(Equal("localhost"))
			})

			It("loads port", func() {
				Ω(config.Port).Should(Equal("3000"))
			})

			It("loads the auth credentials", func() {
				Ω(config.AuthConfiguration.Username).To(Equal("admin"))
				Ω(config.AuthConfiguration.Password).To(Equal("secret"))
			})

			It("loads plans", func() {
				Ω(config.ServiceConfiguration.Plans).To(BeEquivalentTo(
					[]brokerconfig.Plan{
						{
							Name:         "somename",
							ID:           "someid",
							StorageClass: "persistent",
							Free:         true,
							Description:  "this is a description",
						},
						{
							Name:         "someothername",
							ID:           "someotherid",
							StorageClass: "gold",
							Free:         false,
							Description:  "this is another description",
						},
					},
				))
			})
		})

		Context("when the configuration is invalid", func() {

			BeforeEach(func() {
				configPath = "test_config.yml-invalid"
			})

			It("returns an error", func() {
				Ω(parseConfigErr).Should(MatchError(ContainSubstring("cannot unmarshal")))
			})
		})
	})
})
