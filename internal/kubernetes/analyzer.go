package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// FlexibleLabel marks pods that can be rescheduled based on carbon intensity.
	FlexibleLabel = "ecoscale/flexible"
	// FlexibleLabelValue is the value required for the flexible label.
	FlexibleLabelValue = "true"
	// FlexibleSelector is the label selector for flexible pods.
	FlexibleSelector = "ecoscale/flexible=true"
	// ProtectedLabel marks pods that must never be evicted or drained.
	ProtectedLabel = "ecoscale/protected"
)

// PodInfo holds summarized information about a flexible pod.
type PodInfo struct {
	Name      string
	Namespace string
	NodeName  string
	Phase     corev1.PodPhase
	Critical  bool
	Protected bool // ecoscale/protected=true - never evict
}

// Analyzer discovers and analyzes pods eligible for carbon-aware scheduling.
type Analyzer struct {
	clientset kubernetes.Interface
}

// NewAnalyzer creates an Analyzer using in-cluster config.
func NewAnalyzer(config *rest.Config) (*Analyzer, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %w", err)
	}
	return &Analyzer{clientset: clientset}, nil
}

// NewAnalyzerWithClient creates an Analyzer with an existing clientset (for testing).
func NewAnalyzerWithClient(clientset kubernetes.Interface) *Analyzer {
	return &Analyzer{clientset: clientset}
}

// ListFlexiblePods returns all pods with ecoscale/flexible=true across all namespaces.
func (a *Analyzer) ListFlexiblePods(ctx context.Context) ([]PodInfo, error) {
	opts := metav1.ListOptions{
		LabelSelector: FlexibleSelector,
	}
	list, err := a.clientset.CoreV1().Pods("").List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	out := make([]PodInfo, 0, len(list.Items))
	for _, p := range list.Items {
		critical := isCriticalPod(&p)
		protected := isProtectedPod(&p)
		out = append(out, PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			NodeName:  p.Spec.NodeName,
			Phase:     p.Status.Phase,
			Critical:  critical,
			Protected: protected,
		})
	}
	return out, nil
}

// ListFlexiblePodsInNamespace returns flexible pods in a specific namespace.
func (a *Analyzer) ListFlexiblePodsInNamespace(ctx context.Context, namespace string) ([]PodInfo, error) {
	opts := metav1.ListOptions{
		LabelSelector: FlexibleSelector,
	}
	list, err := a.clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
	}

	out := make([]PodInfo, 0, len(list.Items))
	for _, p := range list.Items {
		critical := isCriticalPod(&p)
		protected := isProtectedPod(&p)
		out = append(out, PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			NodeName:  p.Spec.NodeName,
			Phase:     p.Status.Phase,
			Critical:  critical,
			Protected: protected,
		})
	}
	return out, nil
}

// GetCurrentRegion attempts to infer the cluster's cloud region from node labels.
// Returns empty string if not determinable.
func (a *Analyzer) GetCurrentRegion(ctx context.Context) (string, error) {
	nodes, err := a.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodes.Items) == 0 {
		return "", err
	}
	return getRegionFromNode(&nodes.Items[0]), nil
}

// getRegionFromNode extracts region from common cloud provider node labels.
func getRegionFromNode(n *corev1.Node) string {
	// AWS: topology.kubernetes.io/region
	if r, ok := n.Labels["topology.kubernetes.io/region"]; ok && r != "" {
		return r
	}
	// GCP: topology.kubernetes.io/region
	// Azure: topology.kubernetes.io/region
	// Fallback: failure-domain.beta.kubernetes.io/region
	if r, ok := n.Labels["failure-domain.beta.kubernetes.io/region"]; ok && r != "" {
		return r
	}
	return ""
}

// isCriticalPod returns true if the pod is system-critical and should not be drained.
func isCriticalPod(p *corev1.Pod) bool {
	if p.Namespace == "kube-system" {
		return true
	}
	// DaemonSet pods are always critical
	for _, o := range p.OwnerReferences {
		if o.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// isProtectedPod returns true if the pod has ecoscale/protected=true.
func isProtectedPod(p *corev1.Pod) bool {
	if v, ok := p.Labels[ProtectedLabel]; ok && v == "true" {
		return true
	}
	return false
}

// EvictableCarbonPod reports whether a pod may be evicted for carbon optimization.
func EvictableCarbonPod(p *corev1.Pod) bool {
	if p.Status.Phase != corev1.PodRunning {
		return false
	}
	if isCriticalPod(p) || isProtectedPod(p) {
		return false
	}
	v, ok := p.Labels[FlexibleLabel]
	return ok && v == FlexibleLabelValue
}
