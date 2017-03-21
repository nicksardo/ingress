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
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	hc := healthChecks.New(80, false)
	err := healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHttpHealthCheck(namer.BeName(80))
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}

	hc = healthChecks.New(443, true)
	err = healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHttpsHealthCheck(namer.BeName(443))
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
	httpHC := DefaultHealthCheckTemplate(3000, false)
	httpHC.Name = namer.BeName(3000)
	httpHC.RequestPath = "/my-probes-health"
	hcp.CreateHttpHealthCheck(httpHC.ToHttpHealthCheck())

	// Should not fail adding the same type of health check
	hc := healthChecks.New(3000, false)
	err := healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHttpHealthCheck(httpHC.Name)
	if err != nil {
		t.Fatalf("expected the health check to continue existing, err: %v", err)
	}

	// HTTPS
	// Manually insert a health check
	httpsHC := DefaultHealthCheckTemplate(4000, true)
	httpsHC.Name = namer.BeName(4000)
	httpsHC.RequestPath = "/my-probes-health"
	hcp.CreateHttpsHealthCheck(httpsHC.ToHttpsHealthCheck())

	hc = healthChecks.New(4000, true)
	err = healthChecks.Sync(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the health check exists
	_, err = hcp.GetHttpsHealthCheck(httpsHC.Name)
	if err != nil {
		t.Fatalf("expected the health check to continue existing, err: %v", err)
	}
}

func TestHealthCheckDelete(t *testing.T) {
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	// Create HTTP HC for 1234
	hc := DefaultHealthCheckTemplate(1234, false)
	hc.Name = namer.BeName(1234)
	hcp.CreateHttpHealthCheck(hc.ToHttpHealthCheck())

	// Create HTTPS HC for 1234)
	hcp.CreateHttpsHealthCheck(hc.ToHttpsHealthCheck())

	// Delete only HTTP 1234
	err := healthChecks.Delete(1234, false)
	if err != nil {
		t.Errorf("unexpected error when deleting HTTP health check, err: %v", err)
	}

	// Validate port is deleted
	_, err = hcp.GetHttpHealthCheck(hc.Name)
	if !utils.IsHTTPErrorCode(err, http.StatusNotFound) {
		t.Errorf("expected not-found error, actual: %v", err)
	}

	// Validate the wrong port wasn't deleted
	_, err = hcp.GetHttpsHealthCheck(hc.Name)
	if err != nil {
		t.Errorf("unexpected error retrieving known healthcheck: %v", err)
	}

	// Delete only HTTPS 1234
	err = healthChecks.Delete(1234, true)
	if err != nil {
		t.Errorf("unexpected error when deleting HTTPS health check, err: %v", err)
	}
}

func TestHealthCheckGet(t *testing.T) {
	namer := &utils.Namer{}
	hcp := NewFakeHealthCheckProvider()
	healthChecks := NewHealthChecker(hcp, "/", namer)

	// HTTP
	// Manually insert a health check
	httpHC := DefaultHealthCheckTemplate(3000, false)
	httpHC.Name = namer.BeName(3000)
	httpHC.RequestPath = "/my-probes-health"
	hcp.CreateHttpHealthCheck(httpHC.ToHttpHealthCheck())

	// Verify the health check exists
	_, err := healthChecks.Get(3000, false)
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}

	// HTTPS
	// Manually insert a health check
	httpsHC := DefaultHealthCheckTemplate(4000, true)
	httpsHC.Name = namer.BeName(4000)
	httpsHC.RequestPath = "/my-probes-health"
	hcp.CreateHttpsHealthCheck(httpsHC.ToHttpsHealthCheck())

	// Verify the health check exists
	_, err = healthChecks.Get(4000, true)
	if err != nil {
		t.Fatalf("expected the health check to exist, err: %v", err)
	}
}
