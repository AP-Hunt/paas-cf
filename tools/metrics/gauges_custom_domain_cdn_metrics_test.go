package main_test

import (
	"time"
	"fmt"

	. "github.com/alphagov/paas-cf/tools/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type CloudFrontServiceStub struct {
	domains []CustomDomain
}

func (cf *CloudFrontServiceStub) CustomDomains() ([]CustomDomain, error) {
	return cf.domains, nil
}

var _ = Describe("GaugesCustomDomainCDNMetrics", func() {
	Context("with a single domain", func(){
		It("returns the 6 metrics for the cloudfront domain", func() {

			cloudFrontStub := CloudFrontServiceStub{
				domains: []CustomDomain{
					CustomDomain{
						CloudFrontDomain: "foo.bar.cloudapps.digital",
						AliasDomain:      "foo.bar.gov.uk",
						DistributionId:   "dist-1",
					},
				},
			}

			reader := CustomDomainCDNMetricsCollector(&cloudFrontStub, 15*time.Second)
			defer reader.Close()

			var metrics []Metric
			Eventually(func() int {
				metric, err := reader.ReadMetric()
				if err == nil {
					metrics = append(metrics, metric)
				}
				return len(metrics)
			}, 3*time.Second).Should(BeNumerically(">=", 6))

			Expect(metrics[0].Name).To(Equal("Requests"))
			Expect(metrics[1].Name).To(Equal("BytesDownloaded"))
			Expect(metrics[2].Name).To(Equal("BytesUploaded"))
			Expect(metrics[3].Name).To(Equal("TotalErrorRate"))
			Expect(metrics[4].Name).To(Equal("4xxErrorRate"))
			Expect(metrics[5].Name).To(Equal("5xxErrorRate"))
		})

		It("tags the 6 metrics with the distribution id", func() {

			cloudFrontStub := CloudFrontServiceStub{
				domains: []CustomDomain{
					CustomDomain{
						CloudFrontDomain: "foo.bar.cloudapps.digital",
						AliasDomain:      "foo.bar.gov.uk",
						DistributionId:   "dist-1",
					},
				},
			}

			reader := CustomDomainCDNMetricsCollector(&cloudFrontStub, 15*time.Second)
			defer reader.Close()

			var metrics []Metric
			Eventually(func() int {
				metric, err := reader.ReadMetric()
				if err == nil {
					metrics = append(metrics, metric)
				}
				return len(metrics)
			}, 3*time.Second).Should(BeNumerically(">=", 6))

			expected := fmt.Sprintf("distribution_id=%s", "dist-1")
			Expect(metrics[0].Tags).To(ContainElement(expected))
			Expect(metrics[1].Tags).To(ContainElement(expected))
			Expect(metrics[2].Tags).To(ContainElement(expected))
			Expect(metrics[3].Tags).To(ContainElement(expected))
			Expect(metrics[4].Tags).To(ContainElement(expected))
			Expect(metrics[5].Tags).To(ContainElement(expected))
		})
	})

	Context("with 2 or more domains", func(){
		It("returns the 6 metrics per cloudfront domain", func() {
			cloudFrontStub := CloudFrontServiceStub{
				domains: []CustomDomain{
					CustomDomain{
						CloudFrontDomain: "foo.bar.cloudapps.digital",
						AliasDomain:      "foo.bar.gov.uk",
						DistributionId:   "dist-1",
					},

					CustomDomain{
						CloudFrontDomain: "bar.baz.cloudapps.digital",
						AliasDomain:      "bar.baz.gov.uk",
						DistributionId:   "dist-2",
					},
				},
			}

			reader := CustomDomainCDNMetricsCollector(&cloudFrontStub, 15*time.Second)
			defer reader.Close()

			var metrics []Metric
			Eventually(func() int {
				metric, err := reader.ReadMetric()
				if err == nil {
					metrics = append(metrics, metric)
				}
				return len(metrics)
			}, 3*time.Second).Should(BeNumerically(">=", 12))
		})
	})
})
