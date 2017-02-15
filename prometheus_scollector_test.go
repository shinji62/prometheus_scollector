package main_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shinji62/prometheus_scollector/scollector"
)

var _ = Describe("PrometheusScollector", func() {

	var session *gexec.Session
	var c *scollector.ScollectorCollector
	var collectorServer *ghttp.Server

	BeforeEach(func() {
		var err error
		timeExpire := 3 * time.Hour
		c = scollector.NewScollectorCollector(&timeExpire)
		c.SetReplacingTags("instance:deployment_name")
		prometheus.MustRegister(c)
		collectorServer = ghttp.NewServer()
		collectorServer.RouteToHandler("POST", "/api/put", c.HandleScoll)
		collectorServer.RouteToHandler("GET", "/metrics", prometheus.Handler().ServeHTTP)
		command := exec.Command("scollector", "-h="+collectorServer.URL())
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		time.Sleep(3 * time.Second)

		Ω(err).ShouldNot(HaveOccurred())

	})
	Context("With Valid data", func() {
		It("Should return prometheus metrics", func() {
			resp, err := http.Get(collectorServer.URL() + "/metrics")
			var body bytes.Buffer
			body.Reset()
			_, err = io.Copy(&body, resp.Body)
			_ = resp.Body.Close()
			Expect(err).ToNot(HaveOccurred())
			Expect(body.String()).To(ContainSubstring("cpu_idle"))

		})

		It("Should exchange labels", func() {
			resp, err := http.Get(collectorServer.URL() + "/metrics")
			var body bytes.Buffer
			body.Reset()
			_, err = io.Copy(&body, resp.Body)
			_ = resp.Body.Close()
			fmt.Printf(body.String())
			Expect(err).ToNot(HaveOccurred())
			Expect(body.String()).ToNot(ContainSubstring("instance"))

		})

	})

	AfterEach(func() {
		prometheus.Unregister(c)
		collectorServer.Close()
		session.Kill()
	})

})
