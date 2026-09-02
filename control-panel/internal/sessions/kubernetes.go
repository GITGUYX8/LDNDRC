package sessions

import (
	"context"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var (
	deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	serviceGVR   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	pvcGVR       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}
)

// KubernetesProvisioner adapts the reference project's session template to
// one Deployment, Service, and home PVC per user session.
type KubernetesProvisioner struct {
	client      dynamic.Interface
	namespace   string
	image       string
	storageSize string
	gpuLimit    string
}

func NewKubernetesProvisioner(config *rest.Config, namespace, image string) (*KubernetesProvisioner, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return &KubernetesProvisioner{
		client:      client,
		namespace:   namespace,
		image:       image,
		storageSize: envOr("SESSION_HOME_STORAGE", "5Gi"),
		gpuLimit:    envOr("SESSION_GPU_LIMIT", "1"),
	}, nil
}

// NewInClusterProvisioner creates a provisioner when the control panel runs
// inside Kubernetes. Local development can continue using the in-memory store.
func NewInClusterProvisioner(namespace, image string) (*KubernetesProvisioner, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster Kubernetes config: %w", err)
	}
	return NewKubernetesProvisioner(config, namespace, image)
}

func (p *KubernetesProvisioner) Apply(session Session) error {
	resources := p.resources(session)
	created := make([]schema.GroupVersionResource, 0, len(resources))
	for _, resource := range resources {
		gvr := resourceGVR(resource.GetKind())
		if _, err := p.resource(gvr).Create(context.Background(), resource, metav1.CreateOptions{}); err != nil {
			for _, previous := range created {
				_ = p.resource(previous).Delete(context.Background(), resourceName(previous, session), metav1.DeleteOptions{})
			}
			return fmt.Errorf("create %s %s: %w", resource.GetKind(), resource.GetName(), err)
		}
		created = append(created, gvr)
	}
	return nil
}

func (p *KubernetesProvisioner) Delete(session Session) error {
	for _, item := range []struct {
		gvr  schema.GroupVersionResource
		name string
	}{
		{deploymentGVR, session.WorkloadName},
		{serviceGVR, session.ServiceName},
		{pvcGVR, session.WorkloadName + "-home"},
	} {
		err := p.resource(item.gvr).Delete(context.Background(), item.name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s %s: %w", item.gvr.Resource, item.name, err)
		}
	}
	return nil
}

func (p *KubernetesProvisioner) resource(gvr schema.GroupVersionResource) dynamic.ResourceInterface {
	return p.client.Resource(gvr).Namespace(p.namespace)
}

func (p *KubernetesProvisioner) resources(session Session) []*unstructured.Unstructured {
	labels := map[string]interface{}{
		"app.kubernetes.io/name": "ldndrc-ros2-session",
		"app.kubernetes.io/part-of": "ldndrc",
		"ldndrc/session-id": session.ID,
	}

	pvc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name":      session.WorkloadName + "-home",
			"namespace": p.namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"accessModes": []interface{}{"ReadWriteOnce"},
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{"storage": p.storageSize},
			},
		},
	}}

	env := []interface{}{
		map[string]interface{}{"name": "POD_NAME", "valueFrom": map[string]interface{}{
			"fieldRef": map[string]interface{}{"fieldPath": "metadata.name"},
		}},
		map[string]interface{}{"name": "ROS_DOMAIN_ID", "value": fmt.Sprint(session.RosDomainID)},
		map[string]interface{}{"name": "ROS_AUTOMATIC_DISCOVERY_RANGE", "value": "LOCALHOST"},
		map[string]interface{}{"name": "SELKIES_PORT", "value": "8080"},
		map[string]interface{}{"name": "NVIDIA_VISIBLE_DEVICES", "value": "all"},
		map[string]interface{}{"name": "NVIDIA_DRIVER_CAPABILITIES", "value": "graphics,utility,compute,video,display"},
	}
	resources := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu": "500m", "memory": "2Gi", "nvidia.com/gpu": p.gpuLimit,
		},
		"limits": map[string]interface{}{
			"cpu": "2", "memory": "4Gi", "nvidia.com/gpu": p.gpuLimit,
		},
	}
	container := map[string]interface{}{
		"name": "ros2", "image": p.image, "imagePullPolicy": "IfNotPresent",
		"env": env, "resources": resources,
		"ports": []interface{}{
			map[string]interface{}{"name": "editor", "containerPort": int64(7682)},
			map[string]interface{}{"name": "desktop", "containerPort": int64(8080)},
			map[string]interface{}{"name": "gzweb", "containerPort": int64(9002)},
		},
		"readinessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{"path": "/", "port": "editor"},
			"initialDelaySeconds": int64(15), "periodSeconds": int64(10),
			"timeoutSeconds": int64(3), "failureThreshold": int64(3),
		},
		"securityContext": map[string]interface{}{"allowPrivilegeEscalation": false, "capabilities": map[string]interface{}{"drop": []interface{}{"ALL"}}},
		"volumeMounts": []interface{}{map[string]interface{}{"name": "home", "mountPath": "/home/student"}},
	}
	deployment := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": session.WorkloadName, "namespace": p.namespace, "labels": labels},
		"spec": map[string]interface{}{
			"replicas": int64(1), "selector": map[string]interface{}{"matchLabels": labels},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"labels": labels},
				"spec": map[string]interface{}{
					"nodeSelector": map[string]interface{}{"node-role.kubernetes.io/role": "host"},
					"securityContext": map[string]interface{}{"runAsNonRoot": true, "runAsUser": int64(1000), "runAsGroup": int64(1000), "fsGroup": int64(1000), "fsGroupChangePolicy": "OnRootMismatch", "seccompProfile": map[string]interface{}{"type": "RuntimeDefault"}},
					"containers": []interface{}{container},
					"volumes": []interface{}{map[string]interface{}{"name": "home", "persistentVolumeClaim": map[string]interface{}{"claimName": session.WorkloadName + "-home"}}},
				},
			},
		},
	}}
	service := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Service",
		"metadata": map[string]interface{}{"name": session.ServiceName, "namespace": p.namespace, "labels": labels},
		"spec": map[string]interface{}{
			"type": "ClusterIP", "selector": labels,
			"ports": []interface{}{
				map[string]interface{}{"name": "editor", "port": int64(7682), "targetPort": "editor"},
				map[string]interface{}{"name": "desktop", "port": int64(8080), "targetPort": "desktop"},
				map[string]interface{}{"name": "gzweb", "port": int64(9002), "targetPort": "gzweb"},
			},
		},
	}}
	return []*unstructured.Unstructured{pvc, deployment, service}
}

func resourceGVR(kind string) schema.GroupVersionResource {
	switch kind {
	case "PersistentVolumeClaim":
		return pvcGVR
	case "Service":
		return serviceGVR
	default:
		return deploymentGVR
	}
}

func resourceName(gvr schema.GroupVersionResource, session Session) string {
	if gvr == pvcGVR {
		return session.WorkloadName + "-home"
	}
	if gvr == serviceGVR {
		return session.ServiceName
	}
	return session.WorkloadName
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
