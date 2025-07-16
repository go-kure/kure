package main

import (
	"os"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/go-kure/kure/internal/k8s"
)

func main() {
	y := printers.YAMLPrinter{}

	depl := k8s.CreateDeployment("myapp", "mynamespace")
	ctr := k8s.CreateContainer("web", "nginx", []string{}, []string{})
	svc := k8s.CreateService("web", "mynamespace")
	k8s.SetServiceSelector(svc, map[string]string{"app": "myapp"})
	ing := k8s.CreateIngress("web", "nginx", "ingress-nginx")
	ctrp := apiv1.ContainerPort{Name: "web", ContainerPort: 8080, Protocol: "TCP"}
	//svcp := apiv1.ServicePort{Name: "web", Protocol: "TCP", Port: 80, TargetPort: intstr.FromInt32(8080)}
	svcp := apiv1.ServicePort{Name: "web", Protocol: "TCP", Port: 80, TargetPort: intstr.FromString("web")}
	rule := k8s.CreateIngressRule("www.example.com")
	pathtype := netv1.PathTypePrefix
	path := k8s.CreateIngressPath("/", &pathtype, "web", "web")

	k8s.AddIngressRulePath(rule, path)
	k8s.AddContainerPort(ctr, ctrp)
	k8s.AddDeploymentContainer(depl, ctr)
	k8s.AddServicePort(svc, svcp)
	k8s.AddIngressRule(ing, rule)

	y.PrintObj(depl, os.Stdout)
	y.PrintObj(svc, os.Stdout)
	y.PrintObj(ing, os.Stdout)
}
