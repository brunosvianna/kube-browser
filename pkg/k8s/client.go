package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type Client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

type PVCInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Status       string `json:"status"`
	Capacity     string `json:"capacity"`
	AccessModes  string `json:"accessModes"`
	StorageClass string `json:"storageClass"`
	MountedBy    string `json:"mountedBy"`
	MountPath    string `json:"mountPath"`
}

type FileInfo struct {
	Name    string `json:"name"`
	Size    string `json:"size"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Path    string `json:"path"`
}

func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset:  clientset,
		restConfig: config,
	}, nil
}

func (c *Client) ListNamespaces(ctx context.Context) ([]string, error) {
	nsList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

func (c *Client) ListPVCs(ctx context.Context, namespace string) ([]PVCInfo, error) {
	pvcList, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pvcPodMap := make(map[string]struct {
		podName   string
		mountPath string
	})
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				mountPath := ""
				for _, container := range pod.Spec.Containers {
					for _, mount := range container.VolumeMounts {
						if mount.Name == vol.Name {
							mountPath = mount.MountPath
							break
						}
					}
					if mountPath != "" {
						break
					}
				}
				pvcPodMap[vol.PersistentVolumeClaim.ClaimName] = struct {
					podName   string
					mountPath string
				}{podName: pod.Name, mountPath: mountPath}
			}
		}
	}

	var pvcs []PVCInfo
	for _, pvc := range pvcList.Items {
		capacity := ""
		if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = qty.String()
		}

		accessModes := ""
		for i, mode := range pvc.Spec.AccessModes {
			if i > 0 {
				accessModes += ", "
			}
			accessModes += string(mode)
		}

		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}

		mountedBy := ""
		mountPath := ""
		if info, ok := pvcPodMap[pvc.Name]; ok {
			mountedBy = info.podName
			mountPath = info.mountPath
		}

		pvcs = append(pvcs, PVCInfo{
			Name:         pvc.Name,
			Namespace:    pvc.Namespace,
			Status:       string(pvc.Status.Phase),
			Capacity:     capacity,
			AccessModes:  accessModes,
			StorageClass: storageClass,
			MountedBy:    mountedBy,
			MountPath:    mountPath,
		})
	}

	return pvcs, nil
}

func (c *Client) findPodForPVC(ctx context.Context, namespace, pvcName string) (string, string, error) {
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == pvcName {
				for _, container := range pod.Spec.Containers {
					for _, mount := range container.VolumeMounts {
						if mount.Name == vol.Name {
							return pod.Name, mount.MountPath, nil
						}
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("no running pod found mounting PVC %s", pvcName)
}

func (c *Client) execInPod(ctx context.Context, namespace, podName string, command []string) (string, string, error) {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

func (c *Client) ListFiles(ctx context.Context, namespace, pvcName, path string) ([]FileInfo, error) {
	podName, mountPath, err := c.findPodForPVC(ctx, namespace, pvcName)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(mountPath, path)

	stdout, _, err := c.execInPod(ctx, namespace, podName, []string{
		"ls", "-la", "--time-style=long-iso", fullPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var files []FileInfo
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		name := strings.Join(fields[7:], " ")
		if name == "." || name == ".." {
			continue
		}

		isDir := strings.HasPrefix(fields[0], "d")
		filePath := filepath.Join(path, name)
		if path == "" || path == "/" {
			filePath = name
		}

		files = append(files, FileInfo{
			Name:    name,
			Size:    fields[4],
			ModTime: fields[5] + " " + fields[6],
			IsDir:   isDir,
			Path:    filePath,
		})
	}

	return files, nil
}

func (c *Client) DownloadFile(ctx context.Context, namespace, pvcName, filePath string) (io.Reader, string, error) {
	podName, mountPath, err := c.findPodForPVC(ctx, namespace, pvcName)
	if err != nil {
		return nil, "", err
	}

	fullPath := filepath.Join(mountPath, filePath)
	fileName := filepath.Base(filePath)

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"cat", fullPath},
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return nil, "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %s", stderr.String())
	}

	return &stdout, fileName, nil
}

func (c *Client) UploadFile(ctx context.Context, namespace, pvcName, destPath string, data io.Reader) error {
	podName, mountPath, err := c.findPodForPVC(ctx, namespace, pvcName)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(mountPath, destPath)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, data); err != nil {
		return fmt.Errorf("failed to read upload data: %w", err)
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"tee", fullPath},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  &buf,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %s", stderr.String())
	}

	return nil
}
