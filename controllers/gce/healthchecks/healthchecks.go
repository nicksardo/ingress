/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package healthchecks

import (
	"net/http"

	compute "google.golang.org/api/compute/v1"

	"github.com/golang/glog"

	"k8s.io/ingress/controllers/gce/utils"
)

const (
	// DefaultHealthCheckInterval defines how frequently a probe runs
	DefaultHealthCheckInterval = 60
	// DefaultHealthyThreshold defines the threshold of success probes that declare a backend "healthy"
	DefaultHealthyThreshold = 1
	// DefaultUnhealthyThreshold defines the threshold of failure probes that declare a backend "unhealthy"
	DefaultUnhealthyThreshold = 10
	// DefaultTimeoutSeconds defines the timeout of each probe
	DefaultTimeoutSeconds = 60
)

// HealthChecks manages health checks.
type HealthChecks struct {
	cloud       HealthCheckProvider
	defaultPath string
	namer       *utils.Namer
}

// NewHealthChecker creates a new health checker.
// cloud: the cloud object implementing SingleHealthCheck.
// defaultHealthCheckPath: is the HTTP path to use for health checks.
func NewHealthChecker(cloud HealthCheckProvider, defaultHealthCheckPath string, namer *utils.Namer) HealthChecker {
	return &HealthChecks{cloud, defaultHealthCheckPath, namer}
}

func (h *HealthChecks) New(port int64, protocol utils.AppProtocol) *HealthCheck {
	hc := DefaultHealthCheck(port, protocol)
	hc.Name = h.namer.BeName(port)
	return hc
}

// Sync retrieves a health check based on port, checks type and settings and updates/creates if necessary.
// Sync is only called by the backends.Add func - it's not a pool like other resources.
func (h *HealthChecks) Sync(hc *HealthCheck) (string, error) {
	// Verify default path
	if hc.RequestPath == "" {
		hc.RequestPath = h.defaultPath
	}

	existingHC, err := h.Get(hc.Port)
	if err != nil {
		if !utils.IsHTTPErrorCode(err, http.StatusNotFound) {
			return "", err
		}

		glog.Infof("Creating health check for port %v with protocol %v", hc.Port, hc.Type)
		if err = h.cloud.CreateHealthCheck(hc.Out()); err != nil {
			return "", err
		}

		return h.getHealthCheckLink(hc.Port)
	}

	if existingHC.Protocol() != hc.Protocol() {
		glog.Infof("Updating health check %v because it has protocol %v but need %v", existingHC.Name, existingHC.Type, hc.Type)
		err = h.cloud.UpdateHealthCheck(hc.Out())
		return existingHC.SelfLink, err
	}

	if existingHC.RequestPath != hc.RequestPath {
		// TODO: reconcile health checks, and compare headers interval etc.
		// Currently Ingress doesn't expose all the health check params
		// natively, so some users prefer to hand modify the check.
		glog.Infof("Unexpected request path on health check %v, has %v want %v, NOT reconciling", hc.Name, existingHC.RequestPath, hc.RequestPath)
	} else {
		glog.Infof("Health check %v already exists and has the expected path %v", hc.Name, hc.RequestPath)
	}

	return existingHC.SelfLink, nil
}

func (h *HealthChecks) getHealthCheckLink(port int64) (string, error) {
	hc, err := h.Get(port)
	if err != nil {
		return "", err
	}
	return hc.SelfLink, nil
}

// Delete deletes the health check by port.
func (h *HealthChecks) Delete(port int64) error {
	name := h.namer.BeName(port)
	glog.Infof("Deleting health check %v", name)
	return h.cloud.DeleteHealthCheck(name)
}

// Get returns the health check by port
func (h *HealthChecks) Get(port int64) (*HealthCheck, error) {
	name := h.namer.BeName(port)
	hc, err := h.cloud.GetHealthCheck(name)
	return NewHealthCheck(hc), err
}

func (h *HealthChecks) DeleteLegacy(port int64) error {
	name := h.namer.BeName(port)
	glog.Infof("Deleting legacy HTTP health check %v", name)
	return h.cloud.DeleteHttpHealthCheck(name)
}

// DefaultHealthCheck simply returns the default health check.
func DefaultHealthCheck(port int64, protocol utils.AppProtocol) *HealthCheck {
	httpSettings := compute.HTTPHealthCheck{
		Port: port,
		// Empty string is used as a signal to the caller to use the appropriate
		// default.
		RequestPath: "",
	}

	hcSettings := compute.HealthCheck{
		// How often to health check.
		CheckIntervalSec: DefaultHealthCheckInterval,
		// How long to wait before claiming failure of a health check.
		TimeoutSec: DefaultTimeoutSeconds,
		// Number of healthchecks to pass for a vm to be deemed healthy.
		HealthyThreshold: DefaultHealthyThreshold,
		// Number of healthchecks to fail before the vm is deemed unhealthy.
		UnhealthyThreshold: DefaultUnhealthyThreshold,
		Description:        "Default kubernetes L7 Loadbalancing health check.",
		Type:               string(protocol),
	}

	return &HealthCheck{
		HTTPHealthCheck: httpSettings,
		HealthCheck:     hcSettings,
	}
}

// HealthCheck wraps compute.HealthCheck so consumers aren't worried about checking
// the various HTTP settings duplicated between HttpHealthCheck, HttpsHealthCheck, etc.
type HealthCheck struct {
	compute.HTTPHealthCheck
	compute.HealthCheck
}

// NewHealthCheck creates a HealthCheck which abstracts nested structs away
func NewHealthCheck(inner *compute.HealthCheck) *HealthCheck {
	if inner == nil {
		return nil
	}

	v := &HealthCheck{HealthCheck: *inner}
	switch utils.AppProtocol(inner.Type) {
	case utils.HTTP:
		v.HTTPHealthCheck = *inner.HttpHealthCheck
	case utils.HTTPS:
		v.HTTPHealthCheck = compute.HTTPHealthCheck(*inner.HttpsHealthCheck)
	}

	return v
}

// Protocol returns the type cased to AppProtocol
func (hc *HealthCheck) Protocol() utils.AppProtocol {
	return utils.AppProtocol(hc.Type)
}

// Out returns a valid compute.HealthCheck object
func (hc *HealthCheck) Out() *compute.HealthCheck {
	switch hc.Protocol() {
	case utils.HTTP:
		hc.HealthCheck.HttpsHealthCheck = nil
		hc.HealthCheck.HttpHealthCheck = &hc.HTTPHealthCheck
	case utils.HTTPS:
		https := compute.HTTPSHealthCheck(hc.HTTPHealthCheck)
		hc.HealthCheck.HttpHealthCheck = nil
		hc.HealthCheck.HttpsHealthCheck = &https
	}

	return &hc.HealthCheck
}
