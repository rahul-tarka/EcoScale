package executor

import (
	"context"
	"fmt"
	"log/slog"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/ecoscale/ecoscale/internal/optimizer"
)

// Executor performs actual Kubernetes actions (evict, drain) based on recommendations.
type Executor struct {
	clientset kubernetes.Interface
}

// NewExecutor creates an executor with in-cluster config.
func NewExecutor(config *rest.Config) (*Executor, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &Executor{clientset: clientset}, nil
}

// Execute applies the given recommendations (evict pods, cordon/drain nodes).
func (e *Executor) Execute(ctx context.Context, recs []optimizer.Recommendation) (executed int, err error) {
	for _, r := range recs {
		switch r.Type {
		case optimizer.ActionScaleDown:
			if n, err := e.evictPod(ctx, r.Target); err != nil {
				slog.Error("evict pod failed", "target", r.Target, "error", err)
			} else if n > 0 {
				executed++
			}
		case optimizer.ActionNodeDrain:
			// Node drain: cordon node, then evict pods on it
			// For now we skip - requires node name; recommendation target is generic
			slog.Info("node_drain recommendation (manual or future implementation)", "target", r.Target)
		case optimizer.ActionRegionShift:
			// Region shift is advisory - output Karpenter config, no auto-apply
			slog.Info("region_shift recommendation (advisory)", "target", r.Target)
		}
	}
	return executed, nil
}

// evictPod evicts a pod by namespace/name. Returns 1 if evicted, 0 otherwise.
func (e *Executor) evictPod(ctx context.Context, target string) (int, error) {
	ns, name, err := parsePodTarget(target)
	if err != nil {
		return 0, err
	}

	pod, err := e.clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("get pod: %w", err)
	}

	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{
			GracePeriodSeconds: ptr(int64(30)),
			DryRun:             []string{},
		},
	}

	err = e.clientset.CoreV1().Pods(ns).EvictV1(ctx, eviction)
	if err != nil {
		return 0, err
	}
	slog.Info("evicted pod", "namespace", ns, "name", name)
	return 1, nil
}

func parsePodTarget(target string) (namespace, name string, err error) {
	// Target format: "namespace/name"
	for i := 0; i < len(target); i++ {
		if target[i] == '/' {
			return target[:i], target[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid pod target format: %s (expected namespace/name)", target)
}

func ptr(i int64) *int64 { return &i }
