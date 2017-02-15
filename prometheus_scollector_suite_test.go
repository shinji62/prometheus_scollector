package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPrometheusScollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PrometheusScollector Suite")
}
