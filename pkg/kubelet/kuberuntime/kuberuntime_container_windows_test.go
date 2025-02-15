//go:build windows
// +build windows

/*
Copyright 2022 The Kubernetes Authors.

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

package kuberuntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/kubelet/winstats"
)

func TestApplyPlatformSpecificContainerConfig(t *testing.T) {
	_, _, fakeRuntimeSvc, err := createTestRuntimeManager()
	require.NoError(t, err)

	containerConfig := &runtimeapi.ContainerConfig{}

	resources := v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("128Mi"),
			v1.ResourceCPU:    resource.MustParse("1"),
		},
		Limits: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("256Mi"),
			v1.ResourceCPU:    resource.MustParse("3"),
		},
	}

	gmsaCredSpecName := "gmsa spec name"
	gmsaCredSpec := "credential spec"
	username := "ContainerAdministrator"
	asHostProcess := true
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "bar",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "foo",
					Image:           "busybox",
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         []string{"testCommand"},
					WorkingDir:      "testWorkingDir",
					Resources:       resources,
					SecurityContext: &v1.SecurityContext{
						WindowsOptions: &v1.WindowsSecurityContextOptions{
							GMSACredentialSpecName: &gmsaCredSpecName,
							GMSACredentialSpec:     &gmsaCredSpec,
							RunAsUserName:          &username,
							HostProcess:            &asHostProcess,
						},
					},
				},
			},
		},
	}

	err = fakeRuntimeSvc.applyPlatformSpecificContainerConfig(containerConfig, &pod.Spec.Containers[0], pod, new(int64), "foo", nil)
	require.NoError(t, err)

	limit := int64(3000)
	expectedCpuMax := 10 * limit / int64(winstats.ProcessorCount())
	expectedWindowsConfig := &runtimeapi.WindowsContainerConfig{
		Resources: &runtimeapi.WindowsContainerResources{
			CpuMaximum:         expectedCpuMax,
			MemoryLimitInBytes: 256 * 1024 * 1024,
		},
		SecurityContext: &runtimeapi.WindowsContainerSecurityContext{
			CredentialSpec: gmsaCredSpec,
			RunAsUsername:  "ContainerAdministrator",
			HostProcess:    true,
		},
	}
	assert.Equal(t, expectedWindowsConfig, containerConfig.Windows)
}

func TestCalculateCPUMaximum(t *testing.T) {
	tests := []struct {
		name     string
		cpuLimit resource.Quantity
		cpuCount int64
		want     int64
	}{
		{
			name:     "max range when same amount",
			cpuLimit: resource.MustParse("1"),
			cpuCount: 1,
			want:     10000,
		},
		{
			name:     "percentage calculation is working as intended",
			cpuLimit: resource.MustParse("94"),
			cpuCount: 96,
			want:     9791,
		},
		{
			name:     "half range when half amount",
			cpuLimit: resource.MustParse("1"),
			cpuCount: 2,
			want:     5000,
		},
		{
			name:     "max range when more requested than available",
			cpuLimit: resource.MustParse("2"),
			cpuCount: 1,
			want:     10000,
		},
		{
			name:     "min range when less than minimum",
			cpuLimit: resource.MustParse("1m"),
			cpuCount: 100,
			want:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, calculateCPUMaximum(&tt.cpuLimit, tt.cpuCount))
		})
	}
}
