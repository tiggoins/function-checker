package main

import (
	"bufio"
	"github.com/alecthomas/kingpin/v2"
	"github.com/tiggoins/function-checker/config"
	"github.com/tiggoins/function-checker/resource"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	app := kingpin.New("k8s-function-checker", "create configmap/statefulset/service/ingress "+
		"to test if k8s works fine.").Version("v1.0.0-alpha0")

	cfg := config.CommandArg{}
	app.Flag("namespace", "Namespace to test kubernetes function.").Default("default").
		Short('n').StringVar(&cfg.Namespace)
	app.Flag("ingress-namespace", "Namespace which ingress-nginx located").
		Short('i').Required().StringVar(&cfg.IngressNamespace)
	app.Flag("storageclass", "Storagclass to request storagce").Short('s').
		StringVar(&cfg.Storageclass)
	app.Flag("capacity", "Capacity to create persistencevolume").Short('c').
		Default("50Gi").StringVar(&cfg.Capacity)
	app.Flag("host", "Host to use in ingress").Default("nginx-test.js.sgcc.com.cn").
		Short('h').StringVar(&cfg.Domain)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	checker := config.NewChecker(cfg)
	defer checker.Cancel()

	checker.VerifyFlags()
	
        klog.Infoln("Start to verify k8s function.")
	ingClass := checker.GetDefaultIngressClass()
	ingAnnotate := checker.GetIngressAnnotationValue()
	if cfg.Storageclass == "" {
		cfg.Storageclass = checker.GetDefaultStorageClass()
	}

	rs := new(resource.Operators)
	sts := resource.NewStatefulSet(cfg.Namespace, cfg.Storageclass, apiresource.MustParse(cfg.Capacity))
	rs.Add(resource.NewConfigMap(cfg.Namespace), resource.NewService(cfg.Namespace), sts)
	if ingClass == "" && ingAnnotate == "" {
		klog.Warningf("Cannot find either default ingressclass or --ingress-class flag," +
			"will not create ingress resource.")
	} else {
		rs.Add(resource.NewIngress(cfg.Namespace, ingClass, ingAnnotate, cfg.Domain))
	}

	cleanFunc := func() {
		if err := rs.Delete(checker.Client); err != nil {
			klog.Warningf("Error happened when delete resource: %s", err.Error())
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cleanFunc()
			os.Exit(1)
		}
	}()
	defer func() {
		cleanFunc()
	}()

	var allErrors []error
	err := rs.Create(checker.Client)
	if err != nil {
		klog.Warningf(err.Error())
		allErrors = append(allErrors, err)
	}

	sts.WaitForReady(checker.Client)
	klog.Infoln("Start to test the function of service from internal")

	if sts.AccessFromInternal(checker.Client, checker.RestConf) {
		klog.Infoln("Access service from internal successfully")
	} else {
		klog.Warningln("Access service from internal failed")
		allErrors = append(allErrors, err)
	}

	klog.Infof("Waiting for user to access from browser,ingress domain is [%s]."+
		"Press 'y' if the test was successfully, 'n' if it was not.", cfg.Domain)
	if WaitForUser() {
		klog.Infof("Ingress access test successfully")
	} else {
		klog.Infof("Ingress access test failed")
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		klog.Infoln("Function test with %d errors", len(allErrors))
	} else {
		klog.Infoln("All function test successfully")
	}
}

func WaitForUser() bool {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.EqualFold(input, "y") {
			return true
		} else if strings.EqualFold(input, "n") {
			return false
		} else {
			klog.Info("Invalid input. Please press 'y' if the test was successful, 'n' if it was not:")
		}
	}

	if err := scanner.Err(); err != nil {
		klog.Error(err)
		return false
	}

	return false
}
