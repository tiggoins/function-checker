package resource

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	svcLabels = map[string]string{
		"type":      "service",
		"component": "k8s-function-checker",
	}
)

var _ ResourceOperator = &Service{}

type Service struct {
	svc *corev1.Service
}

func NewService(namespace string) *Service {
	s := new(Service)

	s.svc = &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-svc",
			Namespace: namespace,
			Labels:    svcLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: svcLabels,
			Ports: []corev1.ServicePort{{
				Name:       "function-checker-svc",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.IntOrString{IntVal: 80},
			}},
		},
	}

	return s
}

func (s *Service) FormatedName() string {
	return strings.Join([]string{s.svc.Kind, s.svc.Namespace, s.svc.Name}, "/")
}

func (s *Service) Create(client *kubernetes.Clientset) error {
	co, err := client.CoreV1().Services(s.svc.Namespace).Create(context.Background(), s.svc, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
	}
	klog.Infof("configmap %s/%s create successfully", co.Namespace, co.Name)

	return nil
}

func (s *Service) Delete(client *kubernetes.Clientset) error {
	err := client.CoreV1().Services(s.svc.Namespace).Delete(context.Background(), s.svc.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	} else {
		klog.Infof("configmap %s/%s create successfully", s.svc.Namespace, s.svc.Name)
	}

	return nil
}

func (s *Service) IsExist(client *kubernetes.Clientset) bool {
	_, err := client.CoreV1().Services(s.svc.Namespace).Get(context.Background(), s.svc.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true
		}
	}

	return false
}
