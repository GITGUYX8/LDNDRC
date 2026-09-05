package sessions

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestKubernetesProvisionerCreatesIsolatedSessionResources(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	provisioner := &KubernetesProvisioner{
		client:      client,
		namespace:   "ldndrc",
		image:       "example/workspace:test",
		storageSize: "5Gi",
		gpuLimit:    "1",
	}
	session := Session{
		ID:           "abc123",
		WorkloadName: "ros2-session-abc123",
		ServiceName:  "ros2-session-abc123",
		RosDomainID:  107,
	}

	if err := provisioner.Apply(session); err != nil {
		t.Fatal(err)
	}

	deployment, err := client.Resource(deploymentGVR).Namespace("ldndrc").Get(context.Background(), session.WorkloadName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) != 1 {
		t.Fatalf("container missing: found=%v err=%v", found, err)
	}
	container, found := containers[0].(map[string]interface{})
	if !found {
		t.Fatalf("container missing: found=%v err=%v", found, err)
	}
	env, found, err := unstructured.NestedSlice(container, "env")
	if err != nil || !found {
		t.Fatalf("environment missing: found=%v err=%v", found, err)
	}
	assertEnv(t, env, "ROS_DOMAIN_ID", "107")
	assertEnv(t, env, "ROS_AUTOMATIC_DISCOVERY_RANGE", "LOCALHOST")

	resources, found, err := unstructured.NestedMap(container, "resources", "requests")
	if err != nil || !found || resources["nvidia.com/gpu"] != "1" {
		t.Fatalf("GPU request missing: %#v", resources)
	}

	limits, found, err := unstructured.NestedMap(container, "resources", "limits")
	if err != nil || !found || limits["nvidia.com/gpu"] != "1" {
		t.Fatalf("GPU limit missing: %#v", limits)
	}

	if _, err := client.Resource(serviceGVR).Namespace("ldndrc").Get(context.Background(), session.ServiceName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resource(pvcGVR).Namespace("ldndrc").Get(context.Background(), session.WorkloadName+"-home", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestKubernetesProvisionerOmitsGPUWhenLimitIsZero(t *testing.T) {
	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	provisioner := &KubernetesProvisioner{
		client:      client,
		namespace:   "ldndrc",
		image:       "example/workspace:test",
		storageSize: "5Gi",
		gpuLimit:    "0",
	}
	session := Session{
		ID:           "gpuzero1",
		WorkloadName: "ros2-session-gpuzero1",
		ServiceName:  "ros2-session-gpuzero1",
		RosDomainID:  108,
	}

	if err := provisioner.Apply(session); err != nil {
		t.Fatal(err)
	}

	deployment, err := client.Resource(deploymentGVR).Namespace("ldndrc").Get(context.Background(), session.WorkloadName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) != 1 {
		t.Fatalf("container missing: found=%v err=%v", found, err)
	}
	container, found := containers[0].(map[string]interface{})
	if !found {
		t.Fatalf("container missing: found=%v err=%v", found, err)
	}
	for _, path := range []string{"requests", "limits"} {
		resources, found, err := unstructured.NestedMap(container, "resources", path)
		if err != nil || !found {
			t.Fatalf("resources %s missing: found=%v err=%v", path, found, err)
		}
		if _, ok := resources["nvidia.com/gpu"]; ok {
			t.Fatalf("unexpected GPU %s: %#v", path, resources)
		}
		if resources["cpu"] == "" || resources["memory"] == "" {
			t.Fatalf("CPU/memory %s missing: %#v", path, resources)
		}
	}
}

func assertEnv(t *testing.T, env []interface{}, name, want string) {
	t.Helper()
	for _, item := range env {
		entry, ok := item.(map[string]interface{})
		if ok && entry["name"] == name && entry["value"] == want {
			return
		}
	}
	t.Fatalf("environment %s=%s not found: %#v", name, want, env)
}
