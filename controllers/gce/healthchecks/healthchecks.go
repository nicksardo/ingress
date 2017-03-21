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

func (h *HealthChecks) New(port int64, encrypted bool) *HealthCheck {
	hc := DefaultHealthCheckTemplate(port, encrypted)
	hc.Name = h.namer.BeName(port)
	return hc
}

// Sync retrieves a health check based on port/encryption, checks for properties
// that should be identical, and updates/creates if necessary.
// Sync is only called by the backends.Add func - it's not a pool like other resources.
func (h *HealthChecks) Sync(hc *HealthCheck) error {
	// Verify default path
	if hc.RequestPath == "" {
		hc.RequestPath = h.defaultPath
	}

	existingHC, err := h.Get(hc.Port, hc.Encrypted)
	if err != nil && !utils.IsHTTPErrorCode(err, http.StatusNotFound) {
		return err
	}
	if existingHC == nil {
		glog.Infof("Creating %v health check %v", utils.GetHTTPScheme(hc.Encrypted), hc.Name)
		if hc.Encrypted {
			return h.cloud.CreateHttpsHealthCheck(hc.ToHttpsHealthCheck())
		}
		return h.cloud.CreateHttpHealthCheck(hc.ToHttpHealthCheck())
	}
	if existingHC != nil && existingHC.RequestPath != hc.RequestPath {
		// TODO: reconcile health checks, and compare headers interval etc.
		// Currently Ingress doesn't expose all the health check params
		// natively, so some users prefer to hand modify the check.
		glog.Infof("Unexpected request path on health check %v, has %v want %v, NOT reconciling",
			hc.Name, existingHC.RequestPath, hc.RequestPath)
	} else {
		glog.Infof("Health check %v already exists and has the expected path %v", hc.Name, hc.RequestPath)
	}

	return nil
}

// Delete deletes the health check by port.
func (h *HealthChecks) Delete(port int64, encrypted bool) error {
	scheme := utils.GetHTTPScheme(encrypted)
	name := h.namer.BeName(port)
	glog.Infof("Deleting %v health check %v", scheme, name)
	if encrypted {
		return h.cloud.DeleteHttpsHealthCheck(h.namer.BeName(port))
	}
	return h.cloud.DeleteHttpHealthCheck(h.namer.BeName(port))
}

func (h *HealthChecks) Get(port int64, encrypted bool) (*HealthCheck, error) {
	name := h.namer.BeName(port)
	if encrypted {
		hc, err := h.cloud.GetHttpsHealthCheck(name)
		if err != nil {
			return nil, err
		}
		return NewHealthCheckHttps(hc), nil
	}
	hc, err := h.cloud.GetHttpHealthCheck(name)
	if err != nil {
		return nil, err
	}
	return NewHealthCheckHttp(hc), nil
}

// DefaultHealthCheckTemplate simply returns the default health check template.
func DefaultHealthCheckTemplate(port int64, encrypted bool) *HealthCheck {
	return &HealthCheck{
		HttpHealthCheck: compute.HttpHealthCheck{
			Port: port,
			// Empty string is used as a signal to the caller to use the appropriate
			// default.
			RequestPath: "",
			Description: "Default kubernetes L7 Loadbalancing health check.",
			// How often to health check.
			CheckIntervalSec: DefaultHealthCheckInterval,
			// How long to wait before claiming failure of a health check.
			TimeoutSec: DefaultTimeoutSeconds,
			// Number of healthchecks to pass for a vm to be deemed healthy.
			HealthyThreshold: DefaultHealthyThreshold,
			// Number of healthchecks to fail before the vm is deemed unhealthy.
			UnhealthyThreshold: DefaultUnhealthyThreshold,
		},
		Encrypted: encrypted,
	}
}

type HealthCheck struct {
	compute.HttpHealthCheck
	Encrypted bool
}

// NewHealthCheckHttp
func NewHealthCheckHttp(hc *compute.HttpHealthCheck) *HealthCheck {
	if hc == nil {
		return nil
	}

	return &HealthCheck{
		HttpHealthCheck: *hc,
		Encrypted:       false,
	}
}

// NewHealthCheckHttps
func NewHealthCheckHttps(hc *compute.HttpsHealthCheck) *HealthCheck {
	if hc == nil {
		return nil
	}
	h := *hc
	return &HealthCheck{
		HttpHealthCheck: compute.HttpHealthCheck(h),
		Encrypted:       true,
	}
}

// ToHttpHealthCheck should only be called if Encrypted=false
func (hc *HealthCheck) ToHttpHealthCheck() *compute.HttpHealthCheck {
	return &hc.HttpHealthCheck
}

// ToHttpsHealthCheck should only be called if Encrypted=true
func (hc *HealthCheck) ToHttpsHealthCheck() *compute.HttpsHealthCheck {
	ehc := compute.HttpsHealthCheck(hc.HttpHealthCheck)
	return &ehc
}
