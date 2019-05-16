package main

import "time"

func CustomDomainCDNMetricsCollector(
	cloudFront CloudFrontServiceInterface,
	interval time.Duration,
) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		return w.WriteMetrics([]Metric{
			getRequestMetrics(cloudFront),
			getBytesDownloadedMetrics(),
			getBytesUploadedMetrics(),
			getTotalErrorRateMetrics(),
			get4xxErrorRateMetrics(),
			get5xxErrorRateMetrics(),
		})
	})

}

func getRequestMetrics(cloudFront CloudFrontServiceInterface) Metric {
	domains := CustomDomains()

	for _, domain := range domains; {
		CloudFrontServiceInterface
	}
	return Metric{
		ID:    "bar",
		Kind:  Counter,
		Name:  "Requests",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}

func getBytesDownloadedMetrics() Metric {
	return Metric{
		ID:    "bar",
		Kind:  Counter,
		Name:  "BytesDownloaded",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}

func getBytesUploadedMetrics() Metric {
	return Metric{
		ID:    "bar",
		Kind:  Counter,
		Name:  "BytesUploaded",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}

func getTotalErrorRateMetrics() Metric {
	return Metric{
		ID:    "bar",
		Kind:  Gauge,
		Name:  "TotalErrorRate",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}

func get4xxErrorRateMetrics() Metric {
	return Metric{
		ID:    "bar",
		Kind:  Gauge,
		Name:  "4xxErrorRate",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}

func get5xxErrorRateMetrics() Metric {
	return Metric{
		ID:    "bar",
		Kind:  Gauge,
		Name:  "5xxErrorRate",
		Time:  time.Time{},
		Value: 0,
		Tags:  []string{"distribution_id=dist-1"},
		Unit:  "",
	}
}
