package resource

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	page     = `it works!`
	cmLabels = map[string]string{
		"type":      "configmap",
		"component": "k8s-function-checker",
	}
)

var _ ResourceOperator = &ConfigMap{}

type ConfigMap struct {
	cm *corev1.ConfigMap
}

func NewConfigMap(namespace string) *ConfigMap {
	config := new(ConfigMap)
	config.cm = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-cm",
			Namespace: namespace,
			Labels:    cmLabels,
		},
		Data: map[string]string{
			"index.html": page,
		},
	}

	return config
}

func (c *ConfigMap) FormatedName() string {
	return strings.Join([]string{c.cm.Kind, c.cm.Namespace, c.cm.Name}, "/")
}

func (c *ConfigMap) Create(client *kubernetes.Clientset) error {
	co, err := client.CoreV1().ConfigMaps(c.cm.Namespace).Create(context.Background(), c.cm, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
	}
	klog.Infof("configmap %s/%s create successfully", co.Namespace, co.Name)

	return nil
}

func (c *ConfigMap) Delete(client *kubernetes.Clientset) error {
	err := client.CoreV1().ConfigMaps(c.cm.Namespace).Delete(context.Background(), c.cm.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	} else {
		klog.Infof("configmap %s/%s create successfully", c.cm.Namespace, c.cm.Name)
	}

	return nil
}

func (c *ConfigMap) IsExist(client *kubernetes.Clientset) bool {
	_, err := client.CoreV1().ConfigMaps(c.cm.Namespace).Get(context.Background(), c.cm.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return true
		}
	}

	return false
}
