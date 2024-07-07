package resource

import (
	"bytes"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	//dockerImage = "reg.kolla.org/library/nginx:1.21.4"
	dockerImage = "registry.cn-shanghai.aliyuncs.com/ltzhang/nginx:1.21.4"
	waitTimeout = time.Duration(300) * time.Second
	waitTicker  = time.Duration(2) * time.Second
)

var (
	replicas  = int32(3)
	stsLabels = map[string]string{
		"kind":      "statefulset",
		"component": "k8s-function-checker",
	}
)

var _ OperatorInterface = &StatefulSet{}

type StatefulSet struct {
	sts     *appsv1.StatefulSet
	created bool
}

func NewStatefulSet(namespace, scName string, storageRequest resource.Quantity) *StatefulSet {
	s := new(StatefulSet)

	s.sts = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-sts",
			Namespace: namespace,
			Labels:    stsLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: stsLabels,
			},
			ServiceName: "k8s-function-checker-svc",
			Replicas:    &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: stsLabels,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{{
											Key:      "component",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"k8s-function-checker"},
										}},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							}}}},
					Containers: []corev1.Container{
						{
							Name:  "function-check-container",
							Image: dockerImage,
							Ports: []corev1.ContainerPort{{
								ContainerPort: int32(80),
							}},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "pvc", MountPath: "/opt"},
								{Name: "webpage", MountPath: "/usr/share/nginx/html"},
								{Name: "script", MountPath: "/script"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "webpage",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "k8s-function-checker-cm",
									},
									Items: []corev1.KeyToPath{{Key: "index.html", Path: "index.html"}},
								},
							},
						},
						{
							Name: "script",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "k8s-function-checker-cm",
									},
									Items: []corev1.KeyToPath{{Key: "service-checker.sh", Path: "service-checker.sh",
										Mode: int32Ptr(0755)}},
								},
							},
						},
					},
				},
			},
		},
	}

	s.sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &scName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageRequest,
				},
			},
		},
	}}

	return s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func (s *StatefulSet) FormatedName() string {
	return strings.Join([]string{s.sts.Namespace, "statefulsets", s.sts.Name}, "/")
}

func (s *StatefulSet) Create(client *kubernetes.Clientset) error {
	_, err := client.AppsV1().StatefulSets(s.sts.Namespace).Create(context.Background(), s.sts, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}
	s.created = true

	return nil
}

func (s *StatefulSet) IsCreated() bool {
	return s.created
}

func (s *StatefulSet) Delete(client *kubernetes.Clientset) error {
	err := client.AppsV1().StatefulSets(s.sts.Namespace).Delete(context.Background(), s.sts.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}

	err = client.CoreV1().PersistentVolumeClaims(s.sts.Namespace).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(map[string]string{
			"component": "k8s-function-checker",
		}),
	})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}

	return nil
}

func (s *StatefulSet) IsExist(client *kubernetes.Clientset) bool {
	_, err := client.AppsV1().StatefulSets(s.sts.Namespace).Get(context.Background(), s.sts.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true
		}
	}

	return false
}

func (s *StatefulSet) WaitForReady(client *kubernetes.Clientset) {
	klog.Infof("Waiting for [%s/statefulsets/%s] to become ready", s.sts.Namespace, s.sts.Name)
	startTime := time.Now()

	timeCh := time.After(waitTimeout)
	retryTicker := time.NewTicker(waitTicker)
	defer retryTicker.Stop()

loop:
	for {
		select {
		case <-timeCh:
			klog.Errorln("Waiting timeout")
		case <-retryTicker.C:
			sts, err := client.AppsV1().StatefulSets(s.sts.Namespace).Get(context.Background(), s.sts.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			if *sts.Spec.Replicas == sts.Status.ReadyReplicas {
				klog.Infof("Waiting for [%s] before status become ready", time.Since(startTime).String())
				break loop
			}
		default:
		}
	}
}

func (s *StatefulSet) AccessFromInternal(client *kubernetes.Clientset, rc *rest.Config) bool {
	podName := strings.Join([]string{s.sts.Name, "0"}, "-")

	execCommandName := "/bin/bash /script/service-checker.sh"
	execCommand := fmt.Sprintf("%s %d %s", execCommandName, 3,
		fmt.Sprintf("%s.%s", s.sts.Spec.ServiceName, s.sts.Namespace))
	klog.Infof("Test access from pod [%s] to service [k8s-function-checker-svc],use command [%s]", podName, execCommand)

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(s.sts.Namespace).
		SubResource("exec").
		MaxRetries(3).
		VersionedParams(&corev1.PodExecOptions{
			Container: "function-check-container",
			Command:   strings.Fields(execCommand),
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	klog.V(5).Infof("req.URL()=%s", req.URL().String())
	exec, err := remotecommand.NewSPDYExecutor(rc, "POST", req.URL())
	if err != nil {
		return false
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		klog.Infof("Error occur when execute command in the pod,err=%s", err.Error())
		klog.Infof("Exec stdout=%v", stdout)
		klog.Infof("Exec stderr=%v", stderr)
		return false
	}

	if strings.EqualFold(stdout.String(), strings.Repeat("it works!", 3)) {
		return true
	}

	return false
}
