package checker

import (
	"context"
	"errors"
	"strconv"
	"strings"

	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	IngressClassArg      = "--ingress-class="
	IngressContainerName = "controller"
)

var (
	errIngressClassNotFound      = errors.New("no default IngressClass found in the cluster")
	errIngressAnnotationNotFound = errors.New("no --ingress-class arg found in ingress-nginx pod")
)

type ArgConfig struct {
	Namespace    string
	Storageclass string
	Capacity     string
	Domain       string
}

type Checker struct {
	Client    *kubernetes.Clientset
	Ctx       context.Context
	Cancel    context.CancelFunc
	RestConf  *rest.Config
	argconfig ArgConfig
}

func NewChecker(config ArgConfig) (*Checker, error) {
	rc, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}

	clientset := kubernetes.NewForConfigOrDie(rc)
	ctx, cancel := context.WithCancel(context.Background())

	return &Checker{
		Ctx:       ctx,
		Cancel:    cancel,
		Client:    clientset,
		RestConf:  rc,
		argconfig: config,
	}, nil
}

func (c *Checker) GetDefaultIngressClass() (string, error) {
	ingressClasses, err := c.Client.NetworkingV1().IngressClasses().List(c.Ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningln(err.Error())
		return "", err
	}

	for _, ingressClass := range ingressClasses.Items {
		if val, ok := ingressClass.Annotations[networkingv1beta1.AnnotationIsDefaultIngressClass]; ok {
			return val, nil
		}
	}

	return "", errIngressClassNotFound
}

func (c *Checker) GetIngressAnnotationValue(ns string) (string, error) {
	ingressNginxLabels := map[string]string{
		"app.kubernetes.io/name": "ingress-nginx",
	}

	pods, err := c.Client.CoreV1().Pods(ns).List(c.Ctx, metav1.ListOptions{
		LabelSelector: labels.Set(ingressNginxLabels).AsSelector().String(),
	})
	if err != nil {
		klog.Warningln(err.Error())
		return "", err
	}

	for _, container := range pods.Items[0].Spec.Containers {
		if strings.EqualFold(container.Name, IngressContainerName) {
			for _, arg := range container.Args {
				if strings.HasPrefix(arg, IngressClassArg) {
					return strings.TrimPrefix(arg, IngressClassArg), nil
				}
			}
			return "", errIngressAnnotationNotFound
		}
	}

	return "", errIngressAnnotationNotFound
}

func (c *Checker) VerifyFlags() {
	_, err := c.Client.StorageV1().StorageClasses().Get(c.Ctx, c.argconfig.Storageclass, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Exitf("storagclass %s not found", c.argconfig.Storageclass)
	}

	_, err = c.Client.CoreV1().Namespaces().Get(c.Ctx, c.argconfig.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Exitf("namespace %s not found", c.argconfig.Namespace)
	}

	if !strings.HasSuffix(c.argconfig.Capacity, "Gi") {
		klog.Exit("capacity must be gigabytes,eg., 50Gi")
	}

	capNum := strings.TrimSuffix(c.argconfig.Capacity, "Gi")
	if _, err := strconv.ParseUint(capNum, 10, 32); err != nil {
		klog.Exit("capacity must be positive integer,eg., 50Gi")
	}
}
