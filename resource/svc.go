package resource

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	svcLabels = map[string]string{
		"kind":      "service",
		"component": "k8s-function-checker",
	}
)

var _ OperatorInterface = &Service{}

type Service struct {
	svc     *corev1.Service
	created bool
}

func NewService(namespace string) *Service {
	s := new(Service)

	s.svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-svc",
			Namespace: namespace,
			Labels:    svcLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"component": "k8s-function-checker",
			},
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.IntOrString{IntVal: 80},
			}},
		},
	}

	return s
}

func (s *Service) FormatedName() string {
	return strings.Join([]string{s.svc.Namespace, "services", s.svc.Name}, "/")
}

func (s *Service) Create(client *kubernetes.Clientset) error {
	_, err := client.CoreV1().Services(s.svc.Namespace).Create(context.Background(), s.svc, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}
	s.created = true

	return nil
}

func (s *Service) IsCreated() bool {
	return s.created
}

func (s *Service) Delete(client *kubernetes.Clientset) error {
	err := client.CoreV1().Services(s.svc.Namespace).Delete(context.Background(), s.svc.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}

	return nil
}
