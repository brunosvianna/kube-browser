package k8s

import "context"

type PodExecutor interface {
	execInPod(ctx context.Context, namespace, podName, containerName string, cmd []string) (string, string, error)
	createHelperPod(ctx context.Context, namespace, pvcName, volumeName, nodeName string) (string, error)
	deleteHelperPod(ctx context.Context, namespace, podName string)
}
