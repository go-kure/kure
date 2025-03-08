package main

import (
	"os"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/printers"

	k8s "github.com/go-kure/kure/internal/k8s"
)

func main() {
	depl := k8s.CreateDeployment("myapp")
	ctr := k8s.CreateContainer("web", "nginx", []string{}, []string{})
	svc := k8s.CreateService("web")
	ing := k8s.CreateIngress("web", "nginx")

	ctrp := apiv1.ContainerPort{
		Name:          "web",
		ContainerPort: 8080,
		Protocol:      "TCP",
	}
	svcp := apiv1.ServicePort{
		Name:       "web",
		Protocol:   "TCP",
		Port:       80,
		TargetPort: intstr.FromInt32(8080),
	}
	ingr := netv1.IngressRule{
		Host: "www.example.com",
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Path: "/",
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{
								Name: "web",
								Port: netv1.ServiceBackendPort{
									Name: "web",
								},
							},
						},
					},
				},
			},
		},
	}

	k8s.AddContainerPort(ctr, ctrp)
	k8s.AddDeploymentContainer(depl, ctr)
	k8s.AddServicePort(svc, svcp)
	k8s.AddIngressRule(ing, ingr)

	y := printers.YAMLPrinter{}
	y.PrintObj(depl, os.Stdout)
	y.PrintObj(svc, os.Stdout)
	y.PrintObj(ing, os.Stdout)
}
