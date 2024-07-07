package config

import (
	"context"
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
	storageutil "k8s.io/kubectl/pkg/util/storage"
)

const (
	IngressClassArg      = "--ingress-class="
	IngressContainerName = "controller"
)

type CommandArg struct {
	Namespace        string
	IngressNamespace string
	Storageclass     string
	Capacity         string
	Domain           string
}

type Checker struct {
	flag     CommandArg
	Client   *kubernetes.Clientset
	RestConf *rest.Config
	Ctx      context.Context
	Cancel   context.CancelFunc
}

func NewChecker(cg CommandArg) *Checker {
	rc, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Exitln(err.Error())
	}

	client := kubernetes.NewForConfigOrDie(rc)
	ctx, cancel := context.WithCancel(context.Background())

	return &Checker{
		flag:     cg,
		Ctx:      ctx,
		Cancel:   cancel,
		Client:   client,
		RestConf: rc,
	}
}

func (c *Checker) GetDefaultIngressClass() string {
	ingressClasses, err := c.Client.NetworkingV1().IngressClasses().List(c.Ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningln(err.Error())
		return ""
	}

	for _, ingressClass := range ingressClasses.Items {
		if _, ok := ingressClass.Annotations[networkingv1beta1.AnnotationIsDefaultIngressClass]; ok {
			klog.Infof("Get default ingressclass=[%s]", ingressClass.Name)
			return ingressClass.Name
		}
	}

	return ""
}

func (c *Checker) GetIngressAnnotationValue() string {
	ingressNginxLabels := map[string]string{
		"app.kubernetes.io/name":      "ingress-nginx",
		"app.kubernetes.io/component": "controller",
	}

	_, err := c.Client.CoreV1().Namespaces().Get(c.Ctx, c.flag.IngressNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return ""
	}

	pods, err := c.Client.CoreV1().Pods(c.flag.IngressNamespace).List(c.Ctx, metav1.ListOptions{
		LabelSelector: labels.Set(ingressNginxLabels).AsSelector().String(),
	})
	if err != nil {
		klog.Warningln(err.Error())
		return ""
	}

	for _, container := range pods.Items[0].Spec.Containers {
		if strings.EqualFold(container.Name, IngressContainerName) {
			for _, arg := range container.Args {
				if strings.HasPrefix(arg, IngressClassArg) {
					argValue := arg[len(IngressClassArg):]
					klog.Infof("Get --ingress-class argument value=[%s] in pod [%s]", argValue, pods.Items[0].Name)
					return argValue
				}
			}
			return ""
		}
	}

	return ""
}

func (c *Checker) GetDefaultStorageClass() string {
	storageClasses, err := c.Client.StorageV1().StorageClasses().List(c.Ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningln(err.Error())
		return ""
	}

	for _, storageclass := range storageClasses.Items {
		if _, ok := storageclass.Annotations[storageutil.IsDefaultStorageClassAnnotation]; ok {
			klog.Infof("Get default storageclass=[%s]", storageclass.Name)
			return storageclass.Name
		}
	}

	klog.Exitf("Cannot find default storageclass,please use -s <storageclass> specify which storageclass to use")
	return ""
}

func (c *Checker) VerifyFlags() {
	if c.flag.Namespace == "" {
		klog.Infoln("Namespace is empty,will use `default` namespace")
	}

	_, err := c.Client.StorageV1().StorageClasses().Get(c.Ctx, c.flag.Storageclass, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Exitf("storagclass [%s] not found", c.flag.Storageclass)
	}

	_, err = c.Client.CoreV1().Namespaces().Get(c.Ctx, c.flag.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Exitf("namespace [%s] not found", c.flag.Namespace)
	}

	_, err = c.Client.CoreV1().Namespaces().Get(c.Ctx, c.flag.IngressNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.Exitf("namespace [%s] not found", c.flag.IngressNamespace)
	}

	if !strings.HasSuffix(c.flag.Capacity, "Gi") {
		klog.Exit("capacity must be gigabytes,eg., 50Gi")
	}

	capNum := strings.TrimSuffix(c.flag.Capacity, "Gi")
	if _, err := strconv.ParseUint(capNum, 10, 32); err != nil {
		klog.Exit("capacity must be positive integer,eg., 50Gi")
	}
}
