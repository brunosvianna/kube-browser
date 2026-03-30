package k8s

import (
        "context"
        "errors"
        "fmt"
        "strings"

        apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type ErrorKind string

const (
        ErrKindRBAC          ErrorKind = "RBAC"
        ErrKindNoShell       ErrorKind = "NoShell"
        ErrKindTimeout       ErrorKind = "Timeout"
        ErrKindHelperPending ErrorKind = "HelperPending"
        ErrKindPathNotFound  ErrorKind = "PathNotFound"
        ErrKindPermDenied    ErrorKind = "PermDenied"
        ErrKindUnknown       ErrorKind = "Unknown"
)

type K8sError struct {
        Kind    ErrorKind
        Message string
        Cause   error
}

func (e *K8sError) Error() string {
        if e.Cause != nil {
                return fmt.Sprintf("%s (cause: %v)", e.Message, e.Cause)
        }
        return e.Message
}

func (e *K8sError) Unwrap() error {
        return e.Cause
}

func classifyExecError(err error, stderr string) *K8sError {
        if err == nil {
                return nil
        }

        if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
                return &K8sError{
                        Kind:    ErrKindTimeout,
                        Message: "Operation timed out. The pod may be overloaded or unreachable.",
                        Cause:   err,
                }
        }

        if apierrors.IsForbidden(err) {
                return &K8sError{
                        Kind:    ErrKindRBAC,
                        Message: "Permission denied: your kubeconfig does not have exec rights on this pod. Add the 'pods/exec' RBAC permission.",
                        Cause:   err,
                }
        }

        errStr := strings.ToLower(err.Error())
        stderrLower := strings.ToLower(stderr)

        if strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "is not allowed") {
                return &K8sError{
                        Kind:    ErrKindRBAC,
                        Message: "Permission denied: your kubeconfig does not have exec rights on this pod. Add the 'pods/exec' RBAC permission.",
                        Cause:   err,
                }
        }

        if strings.Contains(stderrLower, "no such file or directory") || strings.Contains(stderrLower, "not found") {
                return &K8sError{
                        Kind:    ErrKindPathNotFound,
                        Message: "Path not found inside container. Verify the PVC is mounted at the expected path.",
                        Cause:   err,
                }
        }

        if strings.Contains(stderrLower, "permission denied") {
                return &K8sError{
                        Kind:    ErrKindPermDenied,
                        Message: "Permission denied reading path inside container.",
                        Cause:   err,
                }
        }

        if strings.Contains(errStr, "executable file not found") ||
                strings.Contains(stderrLower, "executable file not found") ||
                strings.Contains(stderrLower, "no such file or directory") && strings.Contains(errStr, "exec") {
                return &K8sError{
                        Kind:    ErrKindNoShell,
                        Message: "Container has no shell or listing tools. KubeBrowser will use a helper pod automatically.",
                        Cause:   err,
                }
        }

        return &K8sError{
                Kind:    ErrKindUnknown,
                Message: err.Error(),
                Cause:   err,
        }
}

func classifyPodError(phase, reason string) *K8sError {
        phaseLower := strings.ToLower(phase)
        reasonLower := strings.ToLower(reason)

        if phaseLower == "pending" || phaseLower == "" {
                msg := "Helper pod stuck in Pending."
                if strings.Contains(reasonLower, "imagepull") || strings.Contains(reasonLower, "errimagepull") {
                        msg = "Helper pod failed to start: ImagePullBackOff — the cluster cannot pull the helper image. Set HELPER_IMAGE to an image available in your registry."
                } else if strings.Contains(reasonLower, "unschedulable") {
                        msg = "Helper pod stuck in Pending: no node is available to schedule it. Check node resources and taints."
                } else {
                        msg = "Helper pod stuck in Pending. Possible causes: ImagePullBackOff, PodSecurityPolicy blocking alpine:3.19, no node available, or NetworkPolicy restriction."
                }
                return &K8sError{
                        Kind:    ErrKindHelperPending,
                        Message: msg,
                }
        }

        if phaseLower == "failed" {
                msg := "Helper pod failed to start."
                if strings.Contains(reasonLower, "imagepull") || strings.Contains(reasonLower, "errimagepull") {
                        msg = "Helper pod failed: ImagePullBackOff — the cluster cannot pull the helper image. Set HELPER_IMAGE to an accessible image."
                } else if reason != "" {
                        msg = fmt.Sprintf("Helper pod failed to start (reason: %s). Check cluster events for details.", reason)
                }
                return &K8sError{
                        Kind:    ErrKindHelperPending,
                        Message: msg,
                }
        }

        return &K8sError{
                Kind:    ErrKindUnknown,
                Message: fmt.Sprintf("Helper pod entered unexpected state: phase=%s reason=%s", phase, reason),
        }
}

func classifyApiError(err error) *K8sError {
        if err == nil {
                return nil
        }

        if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
                return &K8sError{
                        Kind:    ErrKindTimeout,
                        Message: "Operation timed out connecting to Kubernetes API.",
                        Cause:   err,
                }
        }

        if apierrors.IsForbidden(err) {
                return &K8sError{
                        Kind:    ErrKindRBAC,
                        Message: "Permission denied: your kubeconfig does not have permission to perform this operation. Check your RBAC roles.",
                        Cause:   err,
                }
        }

        return &K8sError{
                Kind:    ErrKindUnknown,
                Message: err.Error(),
                Cause:   err,
        }
}
