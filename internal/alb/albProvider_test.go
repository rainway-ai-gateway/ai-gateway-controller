// Copyright(c) 2026 Beijing Yingfei Networks Technology Co.Ltd.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http: //www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package openapi

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestK8sPoolName(t *testing.T) {
	got, err := k8sPoolName("ns", "svc", "http", "testk8s")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want := "k8s_ns_svc_http_testk8s"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	got, err = k8sPoolName("ns", "svc", "http", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want := "k8s_ns_svc_http"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestK8sPoolName_Folding(t *testing.T) {
	long := strings.Repeat("a", 80)
	got, err := k8sPoolName(long, "svc", "http", "testk8s")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != K8sPoolNameMaxLength {
		t.Fatalf("folded name should be %d chars, got %d (%q)", K8sPoolNameMaxLength, len(got), got)
	}
	if err := validK8sPoolName(got); err != nil {
		t.Fatalf("folded name should be valid: %v", err)
	}

	again, _ := k8sPoolName(long, "svc", "http", "testk8s")
	if got != again {
		t.Fatalf("folding should be deterministic: %q vs %q", got, again)
	}
}

func TestK8sPoolName_Invalid(t *testing.T) {
	if _, err := k8sPoolName("ns", "svc", "http", "bad cluster"); err == nil {
		t.Fatalf("cluster name with whitespace should be rejected")
	}
}

func TestGetK8sPoolInstances(t *testing.T) {
	ep := &corev1.Endpoints{
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}, {IP: "10.0.0.1"}},
				Ports:     []corev1.EndpointPort{{Name: "http", Port: 80}},
			},
			{
				Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
				Ports:     []corev1.EndpointPort{{Name: "http", Port: 80}},
			},
		},
	}

	got := getK8sPoolInstances(ep, "http")
	if len(got) != 2 {
		t.Fatalf("expected 2 deduplicated instances, got %d (%+v)", len(got), got)
	}
	if got[0].Addr != "10.0.0.1" || got[0].Port != 80 {
		t.Fatalf("unexpected first instance: %+v", got[0])
	}
	if got[1].Addr != "10.0.0.2" || got[1].Port != 80 {
		t.Fatalf("unexpected second instance: %+v", got[1])
	}

	if got := getK8sPoolInstances(ep, "grpc"); got == nil || len(got) != 0 {
		t.Fatalf("unknown port should yield an empty non-nil slice, got %+v", got)
	}
}
