package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ecok8s "github.com/ecoscale/ecoscale/internal/kubernetes"
	"github.com/ecoscale/ecoscale/internal/optimizer"
)

// Executor performs actual Kubernetes actions (evict, drain) based on recommendations.
type Executor struct {
	clientset coreclientset.Interface
}

// NewExecutor creates an executor with in-cluster config.
func NewExecutor(config *rest.Config) (*Executor, error) {
	clientset, err := coreclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &Executor{clientset: clientset}, nil
}

// Execute applies recommendations until maxPodEvictions pod evictions are reached.
// Order: node drain first (one cordon, batched evictions), then per-pod scale_down; region_shift is advisory only.
func (e *Executor) Execute(ctx context.Context, recs []optimizer.Recommendation, maxPodEvictions int) (podEvictions int, err error) {
	if maxPodEvictions <= 0 {
		for _, r := range recs {
			if r.Type == optimizer.ActionRegionShift {
				slog.Info("region_shift recommendation (advisory)", "target", r.Target)
			}
		}
		return 0, nil
	}

	remaining := maxPodEvictions

	for _, r := range recs {
		if r.Type != optimizer.ActionNodeDrain || !strings.HasPrefix(r.Target, "node/") {
			continue
		}
		nodeName := strings.TrimPrefix(r.Target, "node/")
		n, err := e.drainNode(ctx, nodeName, &remaining)
		if err != nil {
			slog.Error("drain node failed", "node", nodeName, "error", err)
		}
		podEvictions += n
	}

	for _, r := range recs {
		if remaining <= 0 {
			break
		}
		if r.Type != optimizer.ActionScaleDown {
			continue
		}
		n, err := e.evictPod(ctx, r.Target)
		if err != nil {
			slog.Error("evict pod failed", "target", r.Target, "error", err)
			continue
		}
		if n > 0 {
			podEvictions += n
			remaining -= n
		}
	}

	for _, r := range recs {
		if r.Type == optimizer.ActionRegionShift {
			slog.Info("region_shift recommendation (advisory)", "target", r.Target)
		}
	}

	return podEvictions, nil
}

func (e *Executor) drainNode(ctx context.Context, nodeName string, remaining *int) (int, error) {
	if *remaining <= 0 {
		return 0, nil
	}

	patch := []byte(`{"spec":{"unschedulable":true}}`)
	_, err := e.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return 0, fmt.Errorf("cordon node: %w", err)
	}
	slog.Info("cordoned node", "node", nodeName)

	sel := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	list, err := e.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: sel})
	if err != nil {
		return 0, fmt.Errorf("list pods on node: %w", err)
	}

	evicted := 0
	for i := range list.Items {
		if *remaining <= 0 {
			break
		}
		p := &list.Items[i]
		if !ecok8s.EvictableCarbonPod(p) {
			continue
		}
		ev := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace},
			DeleteOptions: &metav1.DeleteOptions{
				GracePeriodSeconds: ptr(int64(30)),
			},
		}
		err = e.clientset.CoreV1().Pods(p.Namespace).EvictV1(ctx, ev)
		if err != nil {
			if errors.IsTooManyRequests(err) {
				slog.Warn("evict rate limited / PDB", "pod", p.Namespace+"/"+p.Name, "error", err)
				continue
			}
			slog.Error("evict pod on node failed", "pod", p.Namespace+"/"+p.Name, "error", err)
			continue
		}
		slog.Info("evicted pod from node", "node", nodeName, "namespace", p.Namespace, "name", p.Name)
		evicted++
		*remaining--
	}
	return evicted, nil
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

	if !ecok8s.EvictableCarbonPod(pod) {
		return 0, fmt.Errorf("pod not eligible for carbon eviction: %s/%s", ns, name)
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
	for i := 0; i < len(target); i++ {
		if target[i] == '/' {
			return target[:i], target[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid pod target format: %s (expected namespace/name)", target)
}

func ptr(i int64) *int64 { return &i }
