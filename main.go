package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"k8s_function_checker/checker"
	"k8s_function_checker/resource"
)

func main() {
	app := kingpin.New("k8s-function-checker", "create configmap/statefulset/service/ingress to test if k8s function works fine.")

	config := checker.ArgConfig{}
	app.Flag("namespace", "Namespace to test kubernetes function.").Default("default").Short('n').StringVar(&config.Namespace)
	app.Flag("storageclass", "Storagclass to request storagce").Short('s').Required().StringVar(&config.Storageclass)
	app.Flag("capacity", "Capacity to create persistencevolume").Short('c').Default("50Gi").StringVar(&config.Capacity)
	app.Flag("host", "Host to use in ingress").Default("nginx-test.js.sgcc.com.cn").Short('h').StringVar(&config.Domain)

	kingpin.Version("v1.0.0-alpha0")
	kingpin.MustParse(app.Parse(os.Args[1:]))

	checker, err := checker.NewChecker(config)
	defer checker.Cancel()
	if err != nil {
		klog.Fatalln("Error occour in NewChecker()", err)
	}

	checker.VerifyFlags()

	ingclass, err1 := checker.GetDefaultIngressClass()
	ingAnnotate, err2 := checker.GetIngressAnnotationValue(config.Namespace)

	rs := new(resource.ResourceOpetors)
	sts := resource.NewStatefulSet(config.Namespace, config.Storageclass, apiresource.MustParse(config.Capacity))
	rs.Add(resource.NewConfigMap(config.Namespace), resource.NewService(config.Namespace), sts)
	if err1 != nil && err2 != nil {
		klog.Warningf("Error: [%s],will not create ingress.")
	} else {
		rs.Add(resource.NewIngress(config.Namespace, ingclass, ingAnnotate, config.Domain))
	}

	klog.Infoln("Start to verify k8s function.")

	err = rs.Create(checker.Client)
	if err != nil {
		klog.Warningf(err.Error())
	}

	sts.WaitForReady(checker.Client)
	klog.Infoln("Start to test the function of service")

	if sts.AccessFromInternal(checker.Client, checker.RestConf) {
		klog.Infoln("test service from internal successful")
	} else {
		klog.Infoln("test service from internal failed")
	}

	klog.Infof("Waiting for user to access from browser,%s", config.Domain)
	prompt()

	err = sts.Delete(checker.Client)
	if err != nil {
		klog.Warningf(err.Error())
	}
}

func prompt() bool {
	klog.Infoln("-> Press 'y' if the test was successful, 'n' if it was not:")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.EqualFold(input, "y") {
			return true
		} else if strings.EqualFold(input, "n") {
			return false
		} else {
			klog.Infoln("Invalid input. Please press 'y' if the test was successful, 'n' if it was not:")
		}
	}

	if err := scanner.Err(); err != nil {
		klog.Error(err)
		return false
	}

	// This return is redundant, the function will always return within the loop.
	// However, it's good practice to have it in case the loop is somehow bypassed.
	return false
}
