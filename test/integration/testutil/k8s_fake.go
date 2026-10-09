package testutil

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
// The default label ai-gateway-api-name=<DefaultAPIName> is only injected when
// the key is NOT present in s.Labels, so tests can override or omit it.
func BuildService(s ServiceSpec) *corev1.Service {
	labels := map[string]string{}
	for k, v := range s.Labels {
		labels[k] = v
	}
	if _, ok := labels["ai-gateway-api-name"]; !ok {
		labels["ai-gateway-api-name"] = DefaultAPIName
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
