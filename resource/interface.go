package resource

import (
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type ResourceOperator interface {
	FormatedName() string
	Create(client *kubernetes.Clientset) error
	Delete(client *kubernetes.Clientset) error
	IsExist(client *kubernetes.Clientset) bool
}

type ResourceOpetors struct {
	ros []ResourceOperator
}

func (ros *ResourceOpetors) Add(r ...ResourceOperator) {
	ros.ros = append(ros.ros, r...)
}

func (ros *ResourceOpetors) Create(client *kubernetes.Clientset) error {
	allErrs := []error{}
	for _, r := range ros.ros {
		err := r.Create(client)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("error creating resource: %v ", err))
			continue
		}
		klog.Infof("%s create successfully", r.FormatedName())
	}
	return utilerrors.NewAggregate(allErrs)

}

func (ros *ResourceOpetors) Delete(client *kubernetes.Clientset) error {
	allErrs := []error{}
	for _, r := range ros.ros {
		err := r.Delete(client)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("error creating resource: %v ", err))
			continue
		}
		klog.Infof("%s delete successfully", r.FormatedName())
	}

	return utilerrors.NewAggregate(allErrs)
}
