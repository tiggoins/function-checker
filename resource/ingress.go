package resource

import (
	"context"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	annotationKey = "kubernetes.io/ingress.class"
)

var (
	ingressLabels = map[string]string{
		"kind":      "ingress",
		"component": "k8s-function-checker",
	}
	pathType = networkingv1.PathTypePrefix
)

var _ OperatorInterface = &Ingress{}

type Ingress struct {
	ing     *networkingv1.Ingress
	created bool
}

func NewIngress(namespace, class, annotationValue, domain string) *Ingress {
	i := new(Ingress)
	i.ing = &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-ingress",
			Namespace: namespace,
			Labels:    ingressLabels,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: domain,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "k8s-function-checker-svc",
									Port: networkingv1.ServiceBackendPort{
										Number: int32(80),
									},
								},
							},
						}},
					},
				},
			}},
		},
	}

	if class != "" && annotationValue != "" {
		i.ing.Spec.IngressClassName = &class
	} else if class == "" && annotationValue != "" {
		i.ing.Annotations = map[string]string{
			annotationKey: annotationValue,
		}
	}
	return i
}

func (i *Ingress) FormatedName() string {
	return strings.Join([]string{i.ing.Namespace, "ingresses", i.ing.Name}, "/")
}

func (i *Ingress) Create(client *kubernetes.Clientset) error {
	_, err := client.NetworkingV1().Ingresses(i.ing.Namespace).Create(context.Background(), i.ing, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}
	i.created = true

	return nil
}

func (i *Ingress) IsCreated() bool {
	return i.created
}

func (i *Ingress) Delete(client *kubernetes.Clientset) error {
	err := client.NetworkingV1().Ingresses(i.ing.Namespace).Delete(context.Background(), i.ing.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}

	return nil
}
