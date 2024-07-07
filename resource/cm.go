package resource

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	page   = "it works!"
	script = `#!/bin/bash
      times=$1
      service=$2
      for i in $(seq $times);
      do curl -fsSL "$service";
      done`
	cmLabels = map[string]string{
		"kind":      "configmap",
		"component": "k8s-function-checker",
	}
)

var _ OperatorInterface = &ConfigMap{}

type ConfigMap struct {
	cm      *corev1.ConfigMap
	created bool
}

func NewConfigMap(namespace string) *ConfigMap {
	config := new(ConfigMap)
	config.cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-function-checker-cm",
			Namespace: namespace,
			Labels:    cmLabels,
		},
		Data: map[string]string{
			"index.html":         page,
			"service-checker.sh": script,
		},
	}

	return config
}

func (c *ConfigMap) FormatedName() string {
	return strings.Join([]string{c.cm.Namespace, "configmaps", c.cm.Name}, "/")
}

func (c *ConfigMap) Create(client *kubernetes.Clientset) error {
	_, err := client.CoreV1().ConfigMaps(c.cm.Namespace).Create(context.Background(), c.cm, metav1.CreateOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}
	c.created = true

	return nil
}

func (c *ConfigMap) IsCreated() bool {
	return c.created
}

func (c *ConfigMap) Delete(client *kubernetes.Clientset) error {
	err := client.CoreV1().ConfigMaps(c.cm.Namespace).Delete(context.Background(), c.cm.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infoln(err.Error())
		return err
	}

	return nil
}
