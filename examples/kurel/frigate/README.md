# Frigate Kurel Package

This is a Kurel package for deploying [Frigate](https://github.com/blakeblackshear/frigate), a complete and local NVR designed for Home Assistant with AI object detection.

## Overview

Frigate is a complete and local NVR designed for Home Assistant with AI object detection. It uses OpenCV and Tensorflow to perform realtime object detection locally for IP cameras.

## Prerequisites

- Kubernetes cluster
- Coral USB TPU device (for hardware acceleration)
- Node labeled with `coral-usb=true` where the Coral USB is attached
- MQTT broker (for Home Assistant integration)
- Storage provisioner for persistent volumes
- cert-manager (for TLS certificates)

## Installation

### Basic Installation

```bash
kurel build examples/kurel/frigate | kubectl apply -f -
```

### Installation with Custom Values

1. Create a values file `my-values.yaml`:

```yaml
app:
  namespace: my-frigate
  image:
    tag: 0.13.0

service:
  loadBalancerIP: 10.0.0.100

ingress:
  hostname: frigate.mydomain.com

storage:
  size: 200Gi
```

2. Build and deploy:

```bash
kurel build examples/kurel/frigate --values my-values.yaml | kubectl apply -f -
```

### Using Patches

Apply environment-specific patches:

```bash
# For production environment
kurel build examples/kurel/frigate --patch production | kubectl apply -f -

# For development environment
kurel build examples/kurel/frigate --patch development | kubectl apply -f -

# For high availability setup
kurel build examples/kurel/frigate --patch high-availability | kubectl apply -f -
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `app.namespace` | Namespace to deploy into | `frigate` |
| `app.image.repository` | Frigate image repository | `ghcr.io/blakeblackshear/frigate` |
| `app.image.tag` | Frigate image tag | `0.12.0` |
| `service.type` | Service type | `LoadBalancer` |
| `service.loadBalancerIP` | Static IP for LoadBalancer | `172.32.104.0` |
| `ingress.enabled` | Enable ingress | `true` |
| `ingress.hostname` | Hostname for ingress | `frigate.home.vanginderachter.be` |
| `storage.size` | PVC storage size | `100Gi` |
| `storage.storageClass` | Storage class name | `local-path` |
| `mqtt.host` | MQTT broker host | `mqtt-broker-home.mqttbroker` |
| `detector.type` | Detector type | `edgetpu` |
| `detector.device` | Detector device | `usb` |

### Secrets Management

This package requires several secrets to be configured:

1. **MQTT Password**: Create a secret named `mqttuser` with key `FRIGATE_MQTT_PASSWORD`
2. **Camera Passwords**: Create a secret named `frigate-camera-secrets` with keys for each camera
3. **Frigate Plus API Key**: Create a secret named `frigate-secrets` with key `plus-api-key`

Example using kubectl:

```bash
kubectl create secret generic mqttuser \
  --from-literal=FRIGATE_MQTT_PASSWORD=your-mqtt-password \
  -n frigate

kubectl create secret generic frigate-camera-secrets \
  --from-literal=cam0-password=password0 \
  --from-literal=cam1-password=password1 \
  --from-literal=cam2-password=password2 \
  --from-literal=cam3-password=password3 \
  --from-literal=cam360-password=password360 \
  -n frigate
```

For production, consider using SealedSecrets or External Secrets Operator.

## Camera Configuration

The actual camera configuration should be added to the ConfigMap in `resources/configmap.yaml`. Here's an example:

```yaml
cameras:
  front_door:
    ffmpeg:
      inputs:
        - path: rtsp://{FRIGATE_CAM_USERNAME}:{FRIGATE_CAM0_PASSWORD}@192.168.1.100:554/stream1
          roles:
            - detect
            - record
    detect:
      width: 1920
      height: 1080
      fps: 5
    record:
      enabled: true
      retain:
        days: 7
    snapshots:
      enabled: true
```

## Hardware Acceleration

This package is configured to use a Coral USB TPU for hardware acceleration. The deployment will be scheduled on nodes labeled with `coral-usb=true`.

To label a node:

```bash
kubectl label node <node-name> coral-usb=true
```

## Monitoring

The deployment includes liveness and readiness probes to ensure Frigate is running correctly.

## Troubleshooting

### Pod not scheduling
- Ensure a node is labeled with `coral-usb=true`
- Check if the Coral USB device is properly connected

### MQTT connection issues
- Verify MQTT broker is running and accessible
- Check MQTT credentials in secrets

### Storage issues
- Ensure storage class exists and can provision volumes
- Check available storage capacity

## License

This Kurel package is provided as-is. Frigate itself is licensed under the MIT License.