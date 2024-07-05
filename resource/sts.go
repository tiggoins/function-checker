package resource

import (
	"bytes"
	"context"
	"fmt"
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
	dockerImage = "reg.kolla.org/library/nginx:1.21.4"
)

var (
	replicas  = int32(3)
	stsLabels = map[string]string{
		"type":      "statefulset",
		"component": "k8s-function-checker",
	}
)

var _ ResourceOperator = &StatefulSet{}

type StatefulSet struct {
	sts *appsv1.StatefulSet
}

func NewStatefulSet(namespace, scName string, storageRequest resource.Quantity) *StatefulSet {
	s := new(StatefulSet)

	s.sts = &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "StatefulSet",
		},
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
					Containers: []corev1.Container{
						{
							Name:  "function-check-container",
							Image: dockerImage,
							Ports: []corev1.ContainerPort{{
								ContainerPort: int32(80),
							},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "function-checker-pvc",
									MountPath: "/opt",
								},
								{
									Name:      "web",
									MountPath: "/usr/share/nginx/html",
								},
							},
						},
					},
					Volumes: []corev1.Volume{{
						Name: "web",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "k8s-function-checker-cm",
								},
							},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "function-checker-pvc",
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
			},
			},
		},
	}

	return s
}

func (s *StatefulSet) FormatedName() string {
	return strings.Join([]string{s.sts.Kind, s.sts.Namespace, s.sts.Name}, "/")
}

func (s *StatefulSet) Create(client *kubernetes.Clientset) error {
	co, err := client.AppsV1().StatefulSets(s.sts.Namespace).Create(context.Background(), s.sts, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
	}
	klog.Infof("configmap %s/%s create successfully", co.Namespace, co.Name)

	return nil
}

func (s *StatefulSet) Delete(client *kubernetes.Clientset) error {
	err := client.AppsV1().StatefulSets(s.sts.Namespace).Delete(context.Background(), s.sts.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	} else {
		klog.Infof("configmap %s/%s create successfully", s.sts.Namespace, s.sts.Name)
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
	klog.Infoln("Waiting for statefulset %s/%s to become ready", s.sts.Namespace, s.sts.Name)
	startTime := time.Now()

	for {
		sts, err := client.AppsV1().StatefulSets(s.sts.Namespace).Get(context.Background(), s.sts.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if sts.Status.ReadyReplicas == sts.Status.Replicas {
			klog.Infof("Waiting for %s before status become ready", time.Since(startTime).String())
			break
		}

		time.Sleep(time.Duration(2) * time.Second)
	}
}

func (s *StatefulSet) AccessFromInternal(client *kubernetes.Clientset, rc *rest.Config) bool {
	podName := s.sts.Name + "-0"
	curl := "curl -fsSL %s"

	execCommand := fmt.Sprintf("%s %s", curl, s.sts.Spec.ServiceName)
	klog.Infof("test access to service,use command %s", execCommand)

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(s.sts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{execCommand},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(rc, "POST", req.URL())
	if err != nil {
		return false
	}

	var stdout bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
	})

	if bytes.EqualFold(stdout.Bytes(), []byte(page)) && err == nil {
		return true
	}

	return false
}
