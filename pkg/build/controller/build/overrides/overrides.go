package overrides

import (
	"strings"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/openshift-controller-manager/pkg/build/controller/common"
)

type BuildOverrides struct {
	Config *openshiftcontrolplanev1.BuildOverridesConfig
}

// ApplyOverrides applies configured overrides to a build in a build pod
func (b BuildOverrides) ApplyOverrides(pod *corev1.Pod) error {
	if b.Config == nil {
		return nil
	}

	build, err := common.GetBuildFromPod(pod)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Applying overrides to build %s/%s", build.Namespace, build.Name)

	if b.Config.ForcePull != nil {
		if build.Spec.Strategy.DockerStrategy != nil {
			klog.V(5).Infof("Setting docker strategy ForcePull to %t in build %s/%s", *b.Config.ForcePull, build.Namespace, build.Name)
			build.Spec.Strategy.DockerStrategy.ForcePull = *b.Config.ForcePull
		}
		if build.Spec.Strategy.SourceStrategy != nil {
			klog.V(5).Infof("Setting source strategy ForcePull to %t in build %s/%s", *b.Config.ForcePull, build.Namespace, build.Name)
			build.Spec.Strategy.SourceStrategy.ForcePull = *b.Config.ForcePull
		}
		if build.Spec.Strategy.CustomStrategy != nil {
			pullPolicy := corev1.PullIfNotPresent
			if *b.Config.ForcePull {
				pullPolicy = corev1.PullAlways
			}

			err := applyPullPolicyToPod(pod, pullPolicy)
			if err != nil {
				return err
			}

			klog.V(5).Infof("Setting custom strategy ForcePull to %t in build %s/%s", *b.Config.ForcePull, build.Namespace, build.Name)
			build.Spec.Strategy.CustomStrategy.ForcePull = *b.Config.ForcePull
		}
	}

	// Apply label overrides
	for _, lbl := range b.Config.ImageLabels {
		externalLabel := buildv1.ImageLabel{
			Name:  lbl.Name,
			Value: lbl.Value,
		}
		klog.V(5).Infof("Overriding image label %s=%s in build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		overrideLabel(externalLabel, &build.Spec.Output.ImageLabels)
	}

	if len(b.Config.NodeSelector) != 0 && pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = map[string]string{}
	}
	for k, v := range b.Config.NodeSelector {
		if strings.TrimSpace(k) == corev1.LabelOSStable {
			continue
		}
		klog.V(5).Infof("Adding override nodeselector %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Spec.NodeSelector[k] = v
	}

	if len(b.Config.Annotations) != 0 && pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	for k, v := range b.Config.Annotations {
		klog.V(5).Infof("Adding override annotation %s=%s to build pod %s/%s", k, v, pod.Namespace, pod.Name)
		pod.Annotations[k] = v
	}

	// Override Tolerations
	if len(b.Config.Tolerations) != 0 {
		klog.V(5).Infof("Overriding tolerations for pod %s/%s", pod.Namespace, pod.Name)
		pod.Spec.Tolerations = []corev1.Toleration{}
		for _, toleration := range b.Config.Tolerations {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)
		}
	}

	return common.SetBuildInPod(pod, build)
}

func applyPullPolicyToPod(pod *corev1.Pod, pullPolicy corev1.PullPolicy) error {
	for i := range pod.Spec.InitContainers {
		klog.V(5).Infof("Setting ImagePullPolicy to %q on init container %s of pod %s/%s", pullPolicy, pod.Spec.InitContainers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.InitContainers[i].ImagePullPolicy = pullPolicy
	}
	for i := range pod.Spec.Containers {
		klog.V(5).Infof("Setting ImagePullPolicy to %q on container %s of pod %s/%s", pullPolicy, pod.Spec.Containers[i].Name, pod.Namespace, pod.Name)
		pod.Spec.Containers[i].ImagePullPolicy = pullPolicy
	}
	return nil
}

func overrideLabel(overridingLabel buildv1.ImageLabel, buildLabels *[]buildv1.ImageLabel) {
	found := false
	for i, lbl := range *buildLabels {
		if lbl.Name == overridingLabel.Name {
			klog.V(5).Infof("Replacing label %s (original value %q) with new value %q", lbl.Name, lbl.Value, overridingLabel.Value)
			(*buildLabels)[i] = overridingLabel
			found = true
		}
	}
	if !found {
		*buildLabels = append(*buildLabels, overridingLabel)
	}
}
