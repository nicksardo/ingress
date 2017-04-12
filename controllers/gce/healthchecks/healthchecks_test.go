/*
Copyright 2017 The Kubernetes Authors.

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
	"testing"

	"k8s.io/ingress/controllers/gce/utils"
)

func TestHealthCheckAdd(t *testing.T) {
	namer := utils.NewNamer("ABC", "XYZ")
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	hc := healthChecks.New(80, utils.HTTP)
	_, err := healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHealthCheck(namer.BeName(80))
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}

	hc = healthChecks.New(443, utils.HTTPS)
	_, err = healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHealthCheck(namer.BeName(443))
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}
}

func TestHealthCheckAddExisting(t *testing.T) {
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	// HTTP
	// Manually insert a health check
	httpHC := DefaultHealthCheck(3000, utils.HTTP)
	httpHC.Name = namer.BeName(3000)
	httpHC.RequestPath = "/my-probes-health"
	hcp.CreateHealthCheck(httpHC.Out())

	// Should not fail adding the same type of health check
	hc := healthChecks.New(3000, utils.HTTP)
	_, err := healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHealthCheck(httpHC.Name)
	if err != nil {
		t.Fatalf("expected the health check to continue existing, err: %v", err)
	}

	// HTTPS
	// Manually insert a health check
	httpsHC := DefaultHealthCheck(4000, utils.HTTPS)
	httpsHC.Name = namer.BeName(4000)
	httpsHC.RequestPath = "/my-probes-health"
	hcp.CreateHealthCheck(httpsHC.Out())

	hc = healthChecks.New(4000, utils.HTTPS)
	_, err = healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHealthCheck(httpsHC.Name)
	if err != nil {
		t.Fatalf("expected the health check to continue existing, err: %v", err)
	}
}

func TestHealthCheckDelete(t *testing.T) {
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	// Create HTTP HC for 1234
	hc := DefaultHealthCheck(1234, utils.HTTP)
	hc.Name = namer.BeName(1234)
	hcp.CreateHealthCheck(hc.Out())

	// Create HTTPS HC for 1234)
	hc.Type = string(utils.HTTPS)
	hcp.CreateHealthCheck(hc.Out())

	// Delete only HTTP 1234
	err := healthChecks.Delete(1234)
	if err != nil {
		t.Errorf("unexpected error when deleting health check, err: %v", err)
	}

	// Validate port is deleted
	_, err = hcp.GetHealthCheck(hc.Name)
	if !utils.IsHTTPErrorCode(err, http.StatusNotFound) {
		t.Errorf("expected not-found error, actual: %v", err)
	}

	// Delete only HTTP 1234
	err = healthChecks.Delete(1234)
	if err == nil {
		t.Errorf("expected not-found error when deleting health check, err: %v", err)
	}
}

func TestHealthCheckUpdate(t *testing.T) {
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	// HTTP
	// Manually insert a health check
	hc := DefaultHealthCheck(3000, utils.HTTP)
	hc.Name = namer.BeName(3000)
	hc.RequestPath = "/my-probes-health"
	hcp.CreateHealthCheck(hc.Out())

	// Verify the health check exists
	_, err := healthChecks.Get(3000)
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}

	// Change to HTTPS
	hc.Type = string(utils.HTTPS)
	_, err = healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected err while syncing healthcheck, err %v", err)
	}

	// Verify the health check exists
	_, err = healthChecks.Get(3000)
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}

	// Verify the check is now HTTPS
	if hc.Protocol() != utils.HTTPS {
		t.Fatalf("expected check to be of type HTTPS")
	}
}
