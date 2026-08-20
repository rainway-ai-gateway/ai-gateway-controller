package testutil

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	inferenceApi "sigs.k8s.io/gateway-api-inference-extension/api/v1"
)

// PortSpec describes a single Service port (name/port/targetPort) plus the
// Endpoint port used for the same port name.
type PortSpec struct {
	Name       string
	Port       int32
	TargetPort int32
	EndPort    int32 // port used in Endpoints subset (defaults to TargetPort)
}

// ServiceSpec carries the minimal data needed to build a Service that the
// controller will treat as a target (passes LabelFilter) plus its Endpoints.
type ServiceSpec struct {
	Name        string
	Namespace   string
	UID         types.UID
	Labels      map[string]string
	Annotations map[string]string
	Ports       []PortSpec // multi-port; if empty a single "http" is used
	PodIPs      []string
}

// BuildService returns a Service object ready to be seeded into the fake client.
// The default label bfe-product=AI_product is only injected when the key is NOT
// present in s.Labels, so tests can override or deliberately omit it.
func BuildService(s ServiceSpec) *corev1.Service {
	labels := map[string]string{}
	for k, v := range s.Labels {
		labels[k] = v
	}
	if _, ok := labels["bfe-product"]; !ok {
		labels["bfe-product"] = "AI_product"
	}

	ports := s.Ports
	if len(ports) == 0 {
		ports = []PortSpec{{Name: "http", Port: 8080, TargetPort: 80}}
	}

	svcPorts := make([]corev1.ServicePort, 0, len(ports))
	for _, p := range ports {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt32(p.TargetPort),
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   s.Namespace,
			UID:         s.UID,
			Labels:      labels,
			Annotations: s.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports:    svcPorts,
			Selector: map[string]string{"app": "backend"},
		},
	}
}

// BuildEndpoints returns an Endpoints object for the same Service (same ns/name).
func BuildEndpoints(s ServiceSpec) *corev1.Endpoints {
	ports := s.Ports
	if len(ports) == 0 {
		ports = []PortSpec{{Name: "http", Port: 8080, TargetPort: 80}}
	}
	if len(s.PodIPs) == 0 {
		s.PodIPs = []string{"10.244.0.99"}
	}

	addrs := make([]corev1.EndpointAddress, 0, len(s.PodIPs))
	for _, ip := range s.PodIPs {
		addrs = append(addrs, corev1.EndpointAddress{IP: ip})
	}

	epPorts := make([]corev1.EndpointPort, 0, len(ports))
	for _, p := range ports {
		endPort := p.EndPort
		if endPort == 0 {
			if p.TargetPort != 0 {
				endPort = p.TargetPort
			} else {
				endPort = p.Port
			}
		}
		epPorts = append(epPorts, corev1.EndpointPort{Name: p.Name, Port: endPort})
	}

	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: addrs,
				Ports:     epPorts,
			},
		},
	}
}

// BuildPod returns a Pod with the given labels and PodIP. When ready=true the
// Pod carries the PodReady=True condition (and no deletion timestamp), which is
// what the InferencePool discovery relies on to include it as an instance.
func BuildPod(name, ns string, labels map[string]string, podIP string, ready bool) *corev1.Pod {
	condStatus := corev1.ConditionFalse
	if ready {
		condStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "test/app"}},
		},
		Status: corev1.PodStatus{
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: condStatus},
			},
		},
	}
}

// BuildInferencePool returns an InferencePool object with the given selector,
// target ports and endpoint picker reference (a Service with a numbered port).
func BuildInferencePool(name, ns string, selector map[string]string, targetPorts []int32, eppName string, eppPort int32) *inferenceApi.InferencePool {
	labels := make(map[inferenceApi.LabelKey]inferenceApi.LabelValue, len(selector))
	for k, v := range selector {
		labels[inferenceApi.LabelKey(k)] = inferenceApi.LabelValue(v)
	}
	tps := make([]inferenceApi.Port, 0, len(targetPorts))
	for _, p := range targetPorts {
		tps = append(tps, inferenceApi.Port{Number: inferenceApi.PortNumber(p)})
	}
	port := inferenceApi.Port{Number: inferenceApi.PortNumber(eppPort)}
	return &inferenceApi.InferencePool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "inference.networking.k8s.io/v1",
			Kind:       "InferencePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: inferenceApi.InferencePoolSpec{
			Selector:    inferenceApi.LabelSelector{MatchLabels: labels},
			TargetPorts: tps,
			EndpointPickerRef: inferenceApi.EndpointPickerRef{
				Name: inferenceApi.ObjectName(eppName),
				Kind: inferenceApi.Kind("Service"),
				Port: &port,
			},
		},
	}
}
