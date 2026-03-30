package k8s

import (
        "bytes"
        "context"
        "fmt"
        "io"
        "log"
        "os"
        gopath "path"
        "path/filepath"
        "runtime"
        "strconv"
        "strings"
        "time"

        corev1 "k8s.io/api/core/v1"
        "k8s.io/apimachinery/pkg/api/resource"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        apierrors "k8s.io/apimachinery/pkg/api/errors"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/kubernetes/scheme"
        "k8s.io/client-go/rest"
        "k8s.io/client-go/tools/clientcmd"
        "k8s.io/client-go/tools/remotecommand"
)

type Client struct {
        clientset      *kubernetes.Clientset
        restConfig     *rest.Config
        KubeconfigPath string
        ContextName    string
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

type KubeconfigInfo struct {
        Contexts       []ContextInfo `json:"contexts"`
        CurrentContext string        `json:"currentContext"`
}

type ContextInfo struct {
        Name      string `json:"name"`
        Cluster   string `json:"cluster"`
        Namespace string `json:"namespace"`
}

func DefaultKubeconfigPath() string {
        if p := os.Getenv("KUBECONFIG"); p != "" {
                return p
        }
        home, _ := os.UserHomeDir()
        if runtime.GOOS == "windows" {
                return filepath.Join(home, ".kube", "config")
        }
        return filepath.Join(home, ".kube", "config")
}

func ReadKubeconfig(kubeconfigPath string) (*KubeconfigInfo, error) {
        if kubeconfigPath == "" {
                kubeconfigPath = DefaultKubeconfigPath()
        }

        if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
                return nil, fmt.Errorf("kubeconfig file not found: %s", kubeconfigPath)
        }

        config, err := clientcmd.LoadFromFile(kubeconfigPath)
        if err != nil {
                return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
        }

        info := &KubeconfigInfo{
                CurrentContext: config.CurrentContext,
        }

        for name, ctx := range config.Contexts {
                info.Contexts = append(info.Contexts, ContextInfo{
                        Name:      name,
                        Cluster:   ctx.Cluster,
                        Namespace: ctx.Namespace,
                })
        }

        return info, nil
}

func NewClientWithContext(kubeconfigPath, contextName string) (*Client, error) {
        if kubeconfigPath == "" {
                kubeconfigPath = DefaultKubeconfigPath()
        }

        configOverrides := &clientcmd.ConfigOverrides{}
        if contextName != "" {
                configOverrides.CurrentContext = contextName
        }

        config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
                &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
                configOverrides,
        ).ClientConfig()
        if err != nil {
                return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
        }

        clientset, err := kubernetes.NewForConfig(config)
        if err != nil {
                return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
        }

        return &Client{
                clientset:      clientset,
                restConfig:     config,
                KubeconfigPath: kubeconfigPath,
                ContextName:    contextName,
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

type podPVCInfo struct {
        podName       string
        containerName string
        mountPath     string
        volumeName    string
        nodeName      string
}

func (c *Client) findPodForPVC(ctx context.Context, namespace, pvcName string) (*podPVCInfo, error) {
        podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
        if err != nil {
                return nil, err
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
                                                        return &podPVCInfo{
                                                                podName:       pod.Name,
                                                                containerName: container.Name,
                                                                mountPath:     mount.MountPath,
                                                                volumeName:    vol.Name,
                                                                nodeName:      pod.Spec.NodeName,
                                                        }, nil
                                                }
                                        }
                                }
                        }
                }
        }

        return nil, fmt.Errorf("no running pod found mounting PVC %s", pvcName)
}

func (c *Client) execInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, string, error) {
        execOpts := &corev1.PodExecOptions{
                Command:   command,
                Stdout:    true,
                Stderr:    true,
                Container: containerName,
        }

        req := c.clientset.CoreV1().RESTClient().Post().
                Resource("pods").
                Name(podName).
                Namespace(namespace).
                SubResource("exec").
                VersionedParams(execOpts, scheme.ParameterCodec)

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

func getEnvWithDefault(key, defaultVal string) string {
        if v := os.Getenv(key); v != "" {
                return v
        }
        return defaultVal
}

func parseQuantityWithDefault(envKey, defaultVal string) resource.Quantity {
        val := getEnvWithDefault(envKey, defaultVal)
        q, err := resource.ParseQuantity(val)
        if err != nil {
                log.Printf("Warning: invalid quantity %q for %s (using default %s): %v", val, envKey, defaultVal, err)
                return resource.MustParse(defaultVal)
        }
        return q
}

func helperResourceRequirements() corev1.ResourceRequirements {
        return corev1.ResourceRequirements{
                Requests: corev1.ResourceList{
                        corev1.ResourceCPU:    parseQuantityWithDefault("HELPER_CPU_REQUEST", "10m"),
                        corev1.ResourceMemory: parseQuantityWithDefault("HELPER_MEM_REQUEST", "16Mi"),
                },
                Limits: corev1.ResourceList{
                        corev1.ResourceCPU:    parseQuantityWithDefault("HELPER_CPU_LIMIT", "100m"),
                        corev1.ResourceMemory: parseQuantityWithDefault("HELPER_MEM_LIMIT", "64Mi"),
                },
        }
}

func helperSecurityContext() *corev1.SecurityContext {
        readOnly := true
        allowPrivEsc := false
        runAsNonRoot := true

        if v := os.Getenv("HELPER_RUN_AS_ROOT"); v == "true" || v == "1" {
                runAsNonRoot = false
        }

        sc := &corev1.SecurityContext{
                ReadOnlyRootFilesystem:   &readOnly,
                AllowPrivilegeEscalation: &allowPrivEsc,
                RunAsNonRoot:             &runAsNonRoot,
        }

        if uid := os.Getenv("HELPER_RUN_AS_USER"); uid != "" {
                if parsed, err := strconv.ParseInt(uid, 10, 64); err == nil {
                        sc.RunAsUser = &parsed
                }
        }

        return sc
}

func (c *Client) createHelperPod(ctx context.Context, namespace, pvcName, volumeName, nodeName string) (string, error) {
        ts := strconv.FormatInt(time.Now().UnixNano(), 16)
        helperName := fmt.Sprintf("kube-browser-helper-%s-%s", pvcName, ts)

        image := getEnvWithDefault("HELPER_IMAGE", "alpine:3.19")

        startupTimeout := 60 * time.Second
        if v := os.Getenv("HELPER_STARTUP_TIMEOUT_SEC"); v != "" {
                if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
                        startupTimeout = time.Duration(parsed) * time.Second
                }
        }

        log.Printf("Creating helper pod %s on node %s for PVC %s (image: %s)", helperName, nodeName, pvcName, image)

        pod := &corev1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                        Name:      helperName,
                        Namespace: namespace,
                        Labels: map[string]string{
                                "app":        "kube-browser-helper",
                                "managed-by": "kube-browser",
                        },
                },
                Spec: corev1.PodSpec{
                        NodeName: nodeName,
                        Containers: []corev1.Container{
                                {
                                        Name:      "helper",
                                        Image:     image,
                                        Command:   []string{"sleep", "300"},
                                        Resources: helperResourceRequirements(),
                                        SecurityContext: helperSecurityContext(),
                                        VolumeMounts: []corev1.VolumeMount{
                                                {
                                                        Name:      "pvc-data",
                                                        MountPath: "/data",
                                                },
                                        },
                                },
                        },
                        Volumes: []corev1.Volume{
                                {
                                        Name: "pvc-data",
                                        VolumeSource: corev1.VolumeSource{
                                                PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                                                        ClaimName: pvcName,
                                                },
                                        },
                                },
                        },
                        RestartPolicy: corev1.RestartPolicyNever,
                },
        }

        _, err := c.clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
        if err != nil {
                if apierrors.IsForbidden(err) {
                        return "", &K8sError{
                                Kind:    ErrKindRBAC,
                                Message: "Permission denied: your kubeconfig cannot create pods. Add 'pods' create RBAC permission.",
                                Cause:   err,
                        }
                }
                return "", fmt.Errorf("failed to create helper pod: %w", err)
        }

        var lastPhase, lastReason string
        deadline := time.Now().Add(startupTimeout)
        for time.Now().Before(deadline) {
                time.Sleep(2 * time.Second)
                p, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, helperName, metav1.GetOptions{})
                if err != nil {
                        log.Printf("Error polling helper pod %s: %v", helperName, err)
                        continue
                }
                lastPhase = string(p.Status.Phase)
                if len(p.Status.ContainerStatuses) > 0 && p.Status.ContainerStatuses[0].State.Waiting != nil {
                        lastReason = p.Status.ContainerStatuses[0].State.Waiting.Reason
                }
                if p.Status.Phase == corev1.PodRunning {
                        log.Printf("Helper pod %s is running", helperName)
                        return helperName, nil
                }
                if p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
                        go c.deleteHelperPod(context.Background(), namespace, helperName)
                        return "", classifyPodError(string(p.Status.Phase), lastReason)
                }
                log.Printf("Waiting for helper pod %s (phase: %s, reason: %s)", helperName, lastPhase, lastReason)
        }

        go c.deleteHelperPod(context.Background(), namespace, helperName)
        return "", classifyPodError(lastPhase, lastReason)
}

func (c *Client) deleteHelperPod(ctx context.Context, namespace, podName string) {
        deleteTimeout := 60 * time.Second
        if v := os.Getenv("HELPER_DELETE_TIMEOUT_SEC"); v != "" {
                if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
                        deleteTimeout = time.Duration(parsed) * time.Second
                }
        }

        const maxRetries = 3
        var lastErr error
        for attempt := 1; attempt <= maxRetries; attempt++ {
                err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
                if err != nil {
                        if apierrors.IsNotFound(err) {
                                log.Printf("Helper pod %s already deleted", podName)
                                return
                        }
                        log.Printf("Attempt %d: failed to delete helper pod %s: %v", attempt, podName, err)
                        lastErr = err
                        time.Sleep(time.Duration(attempt) * time.Second)
                        continue
                }
                lastErr = nil
                break
        }
        if lastErr != nil {
                log.Printf("Warning: could not issue delete for helper pod %s after %d attempts: %v", podName, maxRetries, lastErr)
                return
        }

        deadline := time.Now().Add(deleteTimeout)
        for time.Now().Before(deadline) {
                time.Sleep(2 * time.Second)
                _, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
                if err != nil {
                        if apierrors.IsNotFound(err) {
                                log.Printf("Helper pod %s confirmed deleted", podName)
                                return
                        }
                        log.Printf("Transient error polling helper pod %s deletion: %v", podName, err)
                        continue
                }
                log.Printf("Waiting for helper pod %s to be fully deleted...", podName)
        }
        log.Printf("Warning: helper pod %s was not confirmed deleted within %s", podName, deleteTimeout)
}

func (c *Client) CleanupOrphanedHelperPods(ctx context.Context) {
        log.Printf("Scanning for orphaned helper pods with label managed-by=kube-browser...")
        podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
                LabelSelector: "managed-by=kube-browser",
        })
        if err != nil {
                log.Printf("Warning: failed to list orphaned helper pods: %v", err)
                return
        }
        if len(podList.Items) == 0 {
                log.Printf("No orphaned helper pods found")
                return
        }
        for _, pod := range podList.Items {
                log.Printf("Deleting orphaned helper pod %s/%s (phase: %s)", pod.Namespace, pod.Name, pod.Status.Phase)
                c.deleteHelperPod(ctx, pod.Namespace, pod.Name)
        }
        log.Printf("Orphaned helper pod cleanup complete (%d pods processed)", len(podList.Items))
}

func (c *Client) listFilesGNUls(ctx context.Context, namespace, podName, containerName, mountPath, path string) ([]FileInfo, error) {
        fullPath := mountPath + "/" + path
        stdout, stderr, err := c.execInPod(ctx, namespace, podName, containerName, []string{
                "ls", "-la", "--time-style=long-iso", fullPath,
        })
        if err != nil {
                if stderr != "" {
                        log.Printf("  stderr: %s", strings.TrimSpace(stderr))
                }
                return nil, classifyExecError(err, stderr)
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
                filePath := path + "/" + name
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

func (c *Client) listFilesBusybox(ctx context.Context, namespace, podName, containerName, mountPath, path string) ([]FileInfo, error) {
        fullPath := mountPath + "/" + path
        stdout, stderr, err := c.execInPod(ctx, namespace, podName, containerName, []string{
                "ls", "-la", fullPath,
        })
        if err != nil {
                if stderr != "" {
                        log.Printf("  stderr: %s", strings.TrimSpace(stderr))
                }
                return nil, classifyExecError(err, stderr)
        }

        var files []FileInfo
        lines := strings.Split(stdout, "\n")
        for _, line := range lines {
                line = strings.TrimSpace(line)
                if line == "" || strings.HasPrefix(line, "total") {
                        continue
                }

                fields := strings.Fields(line)
                if len(fields) < 6 {
                        continue
                }

                isDir := strings.HasPrefix(fields[0], "d")

                var name, size, modTime string
                if len(fields) >= 9 {
                        size = fields[4]
                        modTime = fields[5] + " " + fields[6] + " " + fields[7]
                        name = strings.Join(fields[8:], " ")
                } else if len(fields) >= 8 {
                        size = fields[4]
                        modTime = fields[5] + " " + fields[6]
                        name = strings.Join(fields[7:], " ")
                } else {
                        size = fields[3]
                        modTime = fields[4]
                        name = strings.Join(fields[5:], " ")
                }

                if name == "." || name == ".." || name == "" {
                        continue
                }

                filePath := path + "/" + name
                if path == "" || path == "/" {
                        filePath = name
                }

                files = append(files, FileInfo{
                        Name:    name,
                        Size:    size,
                        ModTime: modTime,
                        IsDir:   isDir,
                        Path:    filePath,
                })
        }

        return files, nil
}

func (c *Client) listFilesFind(ctx context.Context, namespace, podName, containerName, mountPath, path string) ([]FileInfo, error) {
        fullPath := mountPath + "/" + path
        stdout, stderr, err := c.execInPod(ctx, namespace, podName, containerName, []string{
                "sh", "-c", fmt.Sprintf("find '%s' -maxdepth 1 -mindepth 1 -exec stat -c '%%n|%%s|%%Y|%%F' {} \\; 2>/dev/null || find '%s' -maxdepth 1 -mindepth 1 -print", fullPath, fullPath),
        })
        if err != nil {
                if stderr != "" {
                        log.Printf("  stderr: %s", strings.TrimSpace(stderr))
                }
                return nil, classifyExecError(err, stderr)
        }

        var files []FileInfo
        lines := strings.Split(stdout, "\n")
        for _, line := range lines {
                line = strings.TrimSpace(line)
                if line == "" {
                        continue
                }

                parts := strings.SplitN(line, "|", 4)
                if len(parts) == 4 {
                        name := parts[0]
                        if strings.HasPrefix(name, fullPath) {
                                name = strings.TrimPrefix(name, fullPath)
                                name = strings.TrimPrefix(name, "/")
                        }
                        if name == "" || name == "." || name == ".." {
                                continue
                        }
                        isDir := strings.Contains(parts[3], "directory")
                        filePath := path + "/" + name
                        if path == "" || path == "/" {
                                filePath = name
                        }
                        files = append(files, FileInfo{
                                Name:    name,
                                Size:    parts[1],
                                ModTime: parts[2],
                                IsDir:   isDir,
                                Path:    filePath,
                        })
                } else {
                        name := line
                        if strings.HasPrefix(name, fullPath) {
                                name = strings.TrimPrefix(name, fullPath)
                                name = strings.TrimPrefix(name, "/")
                        }
                        if name == "" || name == "." || name == ".." {
                                continue
                        }
                        filePath := path + "/" + name
                        if path == "" || path == "/" {
                                filePath = name
                        }
                        files = append(files, FileInfo{
                                Name:    name,
                                Size:    "0",
                                ModTime: "-",
                                IsDir:   false,
                                Path:    filePath,
                        })
                }
        }

        return files, nil
}

func (c *Client) tryListFiles(ctx context.Context, namespace, podName, containerName, mountPath, path string) ([]FileInfo, error) {
        log.Printf("Trying GNU ls on %s/%s (container: %s, mount: %s)", namespace, podName, containerName, mountPath)

        var collectedErrs []*K8sError

        files, err := c.listFilesGNUls(ctx, namespace, podName, containerName, mountPath, path)
        if err == nil {
                return files, nil
        }
        log.Printf("GNU ls failed: %v", err)
        if k, ok := err.(*K8sError); ok {
                collectedErrs = append(collectedErrs, k)
        }

        log.Printf("Trying BusyBox ls")
        files, err = c.listFilesBusybox(ctx, namespace, podName, containerName, mountPath, path)
        if err == nil {
                return files, nil
        }
        log.Printf("BusyBox ls failed: %v", err)
        if k, ok := err.(*K8sError); ok {
                collectedErrs = append(collectedErrs, k)
        }

        log.Printf("Trying find+stat")
        files, err = c.listFilesFind(ctx, namespace, podName, containerName, mountPath, path)
        if err == nil {
                return files, nil
        }
        log.Printf("find+stat failed: %v", err)
        if k, ok := err.(*K8sError); ok {
                collectedErrs = append(collectedErrs, k)
        }

        if best := mostActionableError(collectedErrs...); best != nil {
                return nil, best
        }
        return nil, err
}

func (c *Client) ListFiles(ctx context.Context, namespace, pvcName, path string) ([]FileInfo, error) {
        path = strings.ReplaceAll(path, "\\", "/")
        path = strings.TrimSuffix(path, "/")
        info, err := c.findPodForPVC(ctx, namespace, pvcName)
        if err != nil {
                return nil, err
        }

        files, err := c.tryListFiles(ctx, namespace, info.podName, info.containerName, info.mountPath, path)
        if err == nil {
                return files, nil
        }

        log.Printf("Direct exec failed, creating helper pod for PVC %s on node %s", pvcName, info.nodeName)
        helperName, helperErr := c.createHelperPod(ctx, namespace, pvcName, info.volumeName, info.nodeName)
        if helperErr != nil {
                log.Printf("Direct exec error was: %v", err)
                return nil, helperErr
        }

        files, helperErr = c.tryListFiles(ctx, namespace, helperName, "helper", "/data", path)

        go c.deleteHelperPod(context.Background(), namespace, helperName)

        if helperErr != nil {
                return nil, fmt.Errorf("failed to list files even with helper pod: %w", helperErr)
        }

        return files, nil
}

func (c *Client) execInPodWithContainer(ctx context.Context, namespace, podName, containerName string, opts *corev1.PodExecOptions) (remotecommand.Executor, error) {
        opts.Container = containerName
        req := c.clientset.CoreV1().RESTClient().Post().
                Resource("pods").
                Name(podName).
                Namespace(namespace).
                SubResource("exec").
                VersionedParams(opts, scheme.ParameterCodec)

        return remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
}

func (c *Client) execInPodStreaming(ctx context.Context, namespace, podName, containerName string, command []string, w io.Writer) error {
        execOpts := &corev1.PodExecOptions{
                Command:   command,
                Stdout:    true,
                Stderr:    true,
                Container: containerName,
        }

        req := c.clientset.CoreV1().RESTClient().Post().
                Resource("pods").
                Name(podName).
                Namespace(namespace).
                SubResource("exec").
                VersionedParams(execOpts, scheme.ParameterCodec)

        exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
        if err != nil {
                return err
        }

        var stderr bytes.Buffer
        err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
                Stdout: w,
                Stderr: &stderr,
        })
        if err != nil {
                if stderr.Len() > 0 {
                        return fmt.Errorf("%w: %s", err, stderr.String())
                }
                return err
        }
        return nil
}

func (c *Client) DownloadFile(ctx context.Context, namespace, pvcName, filePath string) (io.Reader, string, error) {
        filePath = strings.ReplaceAll(filePath, "\\", "/")
        info, err := c.findPodForPVC(ctx, namespace, pvcName)
        if err != nil {
                return nil, "", err
        }

        fullPath := info.mountPath + "/" + filePath
        fileName := gopath.Base(filePath)
        podName := info.podName
        containerName := info.containerName
        nodeName := info.nodeName
        volumeName := info.volumeName

        pr, pw := io.Pipe()

        go func() {
                err := c.execInPodStreaming(ctx, namespace, podName, containerName, []string{"cat", fullPath}, pw)
                if err == nil {
                        pw.Close()
                        return
                }

                log.Printf("Direct download failed, trying helper pod on node %s", nodeName)
                helperName, helperErr := c.createHelperPod(ctx, namespace, pvcName, volumeName, nodeName)
                if helperErr != nil {
                        pw.CloseWithError(fmt.Errorf("download failed: %v", err))
                        return
                }
                defer func() {
                        go c.deleteHelperPod(context.Background(), namespace, helperName)
                }()

                helperPath := "/data/" + filePath
                helperErr = c.execInPodStreaming(ctx, namespace, helperName, "helper", []string{"cat", helperPath}, pw)
                if helperErr != nil {
                        pw.CloseWithError(fmt.Errorf("download failed even with helper pod: %v", helperErr))
                        return
                }
                pw.Close()
        }()

        return pr, fileName, nil
}

func (c *Client) UploadFile(ctx context.Context, namespace, pvcName, destPath string, data io.Reader) error {
        destPath = strings.ReplaceAll(destPath, "\\", "/")
        info, err := c.findPodForPVC(ctx, namespace, pvcName)
        if err != nil {
                return err
        }

        fullPath := info.mountPath + "/" + destPath
        podName := info.podName
        containerName := info.containerName

        exec, execErr := c.execInPodWithContainer(ctx, namespace, podName, containerName, &corev1.PodExecOptions{
                Command: []string{"tee", fullPath},
                Stdin:   true,
                Stdout:  true,
                Stderr:  true,
        })

        if execErr != nil {
                log.Printf("Direct upload failed, trying helper pod on node %s", info.nodeName)
                helperName, helperErr := c.createHelperPod(ctx, namespace, pvcName, info.volumeName, info.nodeName)
                if helperErr != nil {
                        return fmt.Errorf("upload failed: %v", execErr)
                }
                defer func() {
                        go c.deleteHelperPod(context.Background(), namespace, helperName)
                }()

                helperPath := "/data/" + destPath
                exec, execErr = c.execInPodWithContainer(ctx, namespace, helperName, "helper", &corev1.PodExecOptions{
                        Command: []string{"tee", helperPath},
                        Stdin:   true,
                        Stdout:  true,
                        Stderr:  true,
                })
                if execErr != nil {
                        return fmt.Errorf("upload failed even with helper pod: %v", execErr)
                }
        }

        var stderr bytes.Buffer
        err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
                Stdin:  data,
                Stdout: io.Discard,
                Stderr: &stderr,
        })
        if err != nil {
                if stderr.Len() > 0 {
                        return fmt.Errorf("failed to upload file: %w: %s", err, stderr.String())
                }
                return fmt.Errorf("failed to upload file: %w", err)
        }

        return nil
}
