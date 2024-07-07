package resource

import (
	"fmt"
	"k8s.io/klog/v2"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

type OperatorInterface interface {
	FormatedName() string
	Create(client *kubernetes.Clientset) error
	IsCreated() bool
	Delete(client *kubernetes.Clientset) error
}

type Operators struct {
	ops []OperatorInterface
}

func (ops *Operators) Add(r ...OperatorInterface) {
	ops.ops = append(ops.ops, r...)
}

func (ops *Operators) Create(client *kubernetes.Clientset) error {
	var allErrs []error
	for _, r := range ops.ops {
		err := r.Create(client)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("error creating resource: %v ", err))
			continue
		}
		klog.Infof("Resource [%s] create successfully", r.FormatedName())
	}
	return utilerrors.NewAggregate(allErrs)
}

func (ops *Operators) Delete(client *kubernetes.Clientset) error {
	var allErrs []error
	for _, r := range ops.ops {
		{
			if r.IsCreated() {
				err := r.Delete(client)
				if err != nil {
					allErrs = append(allErrs, fmt.Errorf("error creating resource: %v ", err))
					continue
				}
				klog.Infof("Resource [%s] delete successfully", r.FormatedName())
			}
		}
	}

	return utilerrors.NewAggregate(allErrs)
}
