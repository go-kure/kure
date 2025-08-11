// Package appworkload provides generators for creating standard Kubernetes workloads
// (Deployments, StatefulSets, DaemonSets) along with their associated resources
// (Services, Ingresses, PersistentVolumeClaims).
//
// The AppWorkload generator follows the GVK (Group, Version, Kind) pattern:
//   - Group: generators.gokure.dev
//   - Version: v1alpha1
//   - Kind: AppWorkload
//
// Example usage:
//
//	apiVersion: generators.gokure.dev/v1alpha1
//	kind: AppWorkload
//	metadata:
//	  name: my-app
//	  namespace: default
//	spec:
//	  workload: Deployment
//	  replicas: 3
//	  containers:
//	    - name: app
//	      image: nginx:latest
//	      ports:
//	        - containerPort: 80
//	  services:
//	    - name: my-service
//	      type: LoadBalancer
//	      ports:
//	        - port: 80
//	          targetPort: 80
package appworkload
