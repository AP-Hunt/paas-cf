package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/alphagov/paas-cf/tools/metrics/pingdumb"
	"github.com/alphagov/paas-cf/tools/metrics/tlscheck"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

func TLSValidityGauge(logger lager.Logger, addr string, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		if !strings.Contains(addr, ":") {
			addr += ":443"
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return err
		}
		metric := Metric{
			Kind:  Gauge,
			Time:  time.Now(),
			Name:  "tls.certificates.validity",
			Value: float64(0),
			Tags: []string{
				fmt.Sprintf("hostname:%s", host),
			},
		}
		cert, err := tlscheck.GetCertificate(addr)
		if err != nil {
			logger.Info("tls-certificates-validity", lager.Data{
				"addr": addr,
				"err":  err.Error(),
			})
			if tlscheck.IsCertificateError(err) {
				metric.Value = 0
			} else {
				return err
			}
		} else {
			daysUntilExpiry := time.Until(cert.NotAfter).Hours() / 24
			metric.Value = float64(daysUntilExpiry)
		}
		return w.WriteMetrics([]Metric{metric})
	})
}

func QuotaGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		orgs, err := c.cf.ListOrgs()
		if err != nil {
			return err
		}

		reservedMemory := 0
		reservedServices := 0
		allocatedMemory := 0
		allocatedServices := 0
		reservedRoutes := 0

		for _, org := range orgs {
			quota, err := org.Quota()
			if err != nil {
				return err
			}
			reservedMemory += quota.MemoryLimit
			reservedServices += quota.TotalServices
			reservedRoutes += quota.TotalRoutes
		}

		apps, err := c.cf.ListApps()
		if err != nil {
			return err
		}
		for _, app := range apps {
			allocatedMemory += (app.Memory * app.Instances)
		}

		allocatedServices, err = c.CountServiceInstances()
		if err != nil {
			return err
		}

		return w.WriteMetrics([]Metric{
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.quota.services.reserved", // number of services reserved by quotas
				Value: float64(reservedServices),
			},
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.quota.services.allocated", // number of services in use
				Value: float64(allocatedServices),
			},
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.quota.memory.reserved", // memory reserved by org quotas
				Value: float64(reservedMemory),
			},
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.quota.memory.allocated", // memory allocated to apps
				Value: float64(allocatedMemory),
			},
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.quota.routes.reserved", // number of routes reserved
				Value: float64(reservedRoutes),
			},
		})
	})
}

func UserCountGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		// global auditor role cannot use /v2/users
		// so we have to fetch users from each org
		orgs, err := c.cf.ListOrgs()
		if err != nil {
			return err
		}

		userGuids := map[string]bool{}

		for _, org := range orgs {
			users, err := c.OrgUsers(org.Guid)
			if err != nil {
				return err
			}
			for _, u := range users {
				userGuids[u.Guid] = true
			}
		}

		return w.WriteMetrics([]Metric{
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.users.count",
				Value: float64(len(userGuids)),
			},
		})
	})
}

func AppCountGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		apps, err := c.cf.ListApps()
		if err != nil {
			return err
		}
		spaces, err := c.cf.ListSpaces()
		if err != nil {
			return err
		}
		orgs, err := c.cf.ListOrgs()
		if err != nil {
			return err
		}
		orgQuotas, err := c.cf.ListOrgQuotas()
		if err != nil {
			return err
		}

		// Number of relevant apps in
		// - APP_STATE: string of whether each app is "started" or "stopped"
		// - ORG_IS_TRIAL: boolean of whether each app is owned by a trial organisation
		// counters[APP_STATE][ORG_IS_TRIAL]
		counters := map[string]map[bool]int{
			"started": map[bool]int{},
			"stopped": map[bool]int{},
		}
		for _, app := range apps {
			space := findSpace(spaces, app.SpaceGuid)
			if space == nil {
				log.Printf("Space was not found for app %s\n", app.Guid)
				continue
			}
			org := findOrg(orgs, space.OrganizationGuid)
			if org == nil {
				log.Printf("Org was not found for app %s in space %s\n", app.Guid, space.Guid)
				continue
			}
			orgQuota := findOrgQuota(orgQuotas, org.QuotaDefinitionGuid)
			if orgQuota == nil {
				log.Printf("Org Quota was not found for app %s in org %s\n", app.Guid, org.Guid)
				continue
			}
			orgIsTrial := isOrgQuotaTrial(orgQuota)
			if app.State == "STARTED" {
				counters["started"][orgIsTrial]++
			}
			if app.State == "STOPPED" {
				counters["stopped"][orgIsTrial]++
			}
		}

		metrics := []Metric{}
		for state, countByTrial := range counters {
			for orgIsTrial, count := range countByTrial {
				metrics = append(metrics, Metric{
					Kind:  Gauge,
					Time:  time.Now(),
					Name:  "op.apps.count",
					Value: float64(count),
					Tags: []string{
						"state:" + state,
						fmt.Sprintf("trial_org:%t", orgIsTrial),
					},
				})
			}
		}
		return w.WriteMetrics(metrics)
	})
}

func ServiceCountGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		serviceInstances, err := c.cf.ListServiceInstances()
		if err != nil {
			return err
		}
		services, err := c.cf.ListServices()
		if err != nil {
			return nil
		}
		servicePlans, err := c.cf.ListServicePlans()
		if err != nil {
			return nil
		}
		spaces, err := c.cf.ListSpaces()
		if err != nil {
			return err
		}
		orgs, err := c.cf.ListOrgs()
		if err != nil {
			return err
		}
		orgQuotas, err := c.cf.ListOrgQuotas()
		if err != nil {
			return err
		}

		// Number of relevant service instances in
		// - ORG_IS_TRIAL: boolean of whether each instance is owned by a trial organisation
		// - SERVICE_PLAN_IS_FREE: whether the instance's service plan is free
		// - NAME_OF_SERVICE: e.g., "mysql" or "postgres"
		// counters[ORG_IS_TRIAL][SERVICE_PLAN_IS_FREE][NAME_OF_SERVICE]
		counters := map[bool]map[bool]map[string]int{
			true: map[bool]map[string]int{
				true:  map[string]int{},
				false: map[string]int{},
			},
			false: map[bool]map[string]int{
				true:  map[string]int{},
				false: map[string]int{},
			},
		}
		for _, instance := range serviceInstances {
			service := findService(services, instance.ServiceGuid)
			if service == nil {
				log.Printf("Service was not found for service instance %s\n", instance.Guid)
				continue
			}
			if service.Label == "" {
				log.Printf("Service label was empty for service %s and service instance %s\n", service.Guid, instance.Guid)
				continue
			}
			servicePlan := findServicePlan(servicePlans, instance.ServicePlanGuid)
			if servicePlan == nil {
				log.Printf("Error finding service plan for service instance %s: %s\n", instance.Guid, err)
				continue
			}
			space := findSpace(spaces, instance.SpaceGuid)
			if space == nil {
				log.Printf("Space was not found for service instance %s\n", instance.Guid)
				continue
			}
			org := findOrg(orgs, space.OrganizationGuid)
			if org == nil {
				log.Printf("Org was not found for service instance %s in space %s\n", instance.Guid, space.Guid)
				continue
			}
			orgQuota := findOrgQuota(orgQuotas, org.QuotaDefinitionGuid)
			if err != nil {
				log.Printf("Org Quota was not found for service instance %s in org %s\n", instance.Guid, org.Guid)
				continue
			}
			orgIsTrial := isOrgQuotaTrial(orgQuota)
			servicePlanIsFree := isServicePlanFree(servicePlan)
			counters[orgIsTrial][servicePlanIsFree][service.Label]++
		}

		metrics := []Metric{}
		for orgIsTrial, x := range counters {
			for servicePlanIsFree, y := range x {
				for serviceLabel, count := range y {
					metrics = append(metrics, Metric{
						Kind:  Gauge,
						Time:  time.Now(),
						Name:  "op.services.provisioned",
						Value: float64(count),
						Tags: []string{
							"type:" + serviceLabel,
							fmt.Sprintf("trial_org:%t", orgIsTrial),
							fmt.Sprintf("free_service:%t", servicePlanIsFree),
						},
					})
				}
			}
		}
		return w.WriteMetrics(metrics)
	})
}

func OrgCountGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		orgs, err := c.cf.ListOrgs()
		if err != nil {
			return err
		}
		counters := map[string]int{}
		for _, org := range orgs {
			quota, err := org.Quota()
			if err != nil {
				log.Printf("Error finding org quota for org %s: %s\n", org.Guid, err)
				continue
			}
			counters[quota.Name]++
		}
		metrics := []Metric{}
		for name, count := range counters {
			metrics = append(metrics, Metric{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.orgs.count",
				Value: float64(count),
				Tags:  []string{"quota:" + name},
			})
		}
		return w.WriteMetrics(metrics)
	})
}

func ELBNodeFailureCountGauge(logger lager.Logger, config pingdumb.ReportConfig, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		r, err := pingdumb.GetReport(config)
		if err != nil {
			return err
		}
		failures := r.Failures()
		for _, failedCheck := range failures {
			logger.Info("elb-node-failure", lager.Data{
				"addr":  failedCheck.Addr,
				"start": failedCheck.Start.Format(time.RFC3339Nano),
				"stop":  failedCheck.Start.Format(time.RFC3339Nano),
				"err":   failedCheck.Err().Error(),
			})
		}
		return w.WriteMetrics([]Metric{
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "aws.elb.unhealthy_node_count",
				Value: float64(len(failures)),
			},
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "aws.elb.healthy_node_count",
				Value: float64(len(r.Checks) - len(failures)),
			},
		})
	})
}

func SpaceCountGauge(c *Client, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		spaces, err := c.cf.ListSpaces()
		if err != nil {
			return err
		}
		return w.WriteMetrics([]Metric{
			{
				Kind:  Gauge,
				Time:  time.Now(),
				Name:  "op.spaces.count",
				Value: float64(len(spaces)),
			},
		})
	})
}

func EventCountGauge(c *Client, eventType string, interval time.Duration) MetricReadCloser {
	return NewMetricPoller(interval, func(w MetricWriter) error {
		u, err := url.Parse("/v2/events")
		if err != nil {
			return err
		}
		maxAge := time.Now().Add(-1 * interval)
		q := u.Query()
		q.Set("order-direction", "desc")
		q.Set("results-per-page", "100")
		q.Add("q", "type:"+eventType)
		q.Add("q", "timestamp>"+maxAge.Format(time.RFC3339Nano))
		u.RawQuery = q.Encode()
		batchUrl := u.String()
		gauge := Metric{
			Time: time.Now(),
			Kind: Gauge,
			Name: "op.events." + eventType,
		}
		for batchUrl != "" {
			var batch struct {
				NextUrl   string          `json:"next_url"`
				Resources []AppUsageEvent `json:"resources"`
			}
			if err := c.get(batchUrl, &batch); err != nil {
				return err
			}
			for _, res := range batch.Resources {
				if res.MetaData.CreatedAt.Before(maxAge) {
					break
				}
				gauge.Value += 1
			}
			batchUrl = batch.NextUrl
		}
		return w.WriteMetrics([]Metric{gauge})
	})
}

func findService(services []cfclient.Service, guid string) *cfclient.Service {
	for _, service := range services {
		if service.Guid == guid {
			return &service
		}
	}
	return nil
}

func findServicePlan(servicePlans []cfclient.ServicePlan, guid string) *cfclient.ServicePlan {
	for _, servicePlan := range servicePlans {
		if servicePlan.Guid == guid {
			return &servicePlan
		}
	}
	return nil
}

func findSpace(spaces []cfclient.Space, guid string) *cfclient.Space {
	for _, space := range spaces {
		if space.Guid == guid {
			return &space
		}
	}
	return nil
}

func findOrg(orgs []cfclient.Org, guid string) *cfclient.Org {
	for _, org := range orgs {
		if org.Guid == guid {
			return &org
		}
	}
	return nil
}

func findOrgQuota(orgQuotas []cfclient.OrgQuota, guid string) *cfclient.OrgQuota {
	for _, orgQuota := range orgQuotas {
		if orgQuota.Guid == guid {
			return &orgQuota
		}
	}
	return nil
}

// Determine if an organisation is on a trial plan.
func isOrgQuotaTrial(quota *cfclient.OrgQuota) bool {
	return quota.Name == "default"
}

// Determine if a service plan is free.
func isServicePlanFree(plan *cfclient.ServicePlan) bool {
	return plan.Name == "Free"
}
