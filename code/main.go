package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig, masterURL, namespace string
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (out-of-cluster). Leave empty for in-cluster.")
	flag.StringVar(&masterURL, "master", "", "API server address (optional).")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch (empty = all namespaces).")
	flag.Parse()

	cfg, err := buildConfig(masterURL, kubeconfig)
	if err != nil {
		exitf("unable to build kubeconfig: %v", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		exitf("unable to create clientset: %v", err)
	}

	var factory informers.SharedInformerFactory
	if namespace == "" {
		factory = informers.NewSharedInformerFactory(cs, 0) // all namespaces
	} else {
		factory = informers.NewSharedInformerFactoryWithOptions(cs, 0, informers.WithNamespace(namespace))
	}

	podInformer := factory.Core().V1().Pods().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				logf("CREATED  pod %s/%s phase=%s ip=%s node=%s",
					pod.Namespace, pod.Name, pod.Status.Phase, pod.Status.PodIP, pod.Spec.NodeName)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, ok1 := oldObj.(*corev1.Pod)
			newPod, ok2 := newObj.(*corev1.Pod)
			if !ok1 || !ok2 {
				return
			}
			if significantChange(oldPod, newPod) {
				logf("UPDATED  pod %s/%s phase:%s->%s ip:%s->%s node:%s->%s",
					newPod.Namespace, newPod.Name,
					oldPod.Status.Phase, newPod.Status.Phase,
					blankIfEqual(oldPod.Status.PodIP, newPod.Status.PodIP), newPod.Status.PodIP,
					blankIfEqual(oldPod.Spec.NodeName, newPod.Spec.NodeName), newPod.Spec.NodeName)
			}
		},
		DeleteFunc: func(obj interface{}) {
			var pod *corev1.Pod
			switch t := obj.(type) {
			case *corev1.Pod:
				pod = t
			case cache.DeletedFinalStateUnknown:
				if p, ok := t.Obj.(*corev1.Pod); ok {
					pod = p
				}
			}
			if pod != nil {
				logf("DELETED  pod %s/%s", pod.Namespace, pod.Name)
			}
		},
	})

	stop := make(chan struct{})
	defer close(stop)

	factory.Start(stop)
	if !cache.WaitForCacheSync(stop, podInformer.HasSynced) {
		exitf("cache did not sync")
	}

	logf("watching pods (namespace=%s, started=%s)", nsOrAll(namespace), time.Now().Format(time.RFC3339))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logf("shutting down")
}

func buildConfig(masterURL, kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	// fallback to default kubeconfig on dev machines
	return clientcmd.BuildConfigFromFlags(masterURL, clientcmd.RecommendedHomeFile)
}

func significantChange(oldPod, newPod *corev1.Pod) bool {
	if oldPod.Status.Phase != newPod.Status.Phase ||
		oldPod.Status.PodIP != newPod.Status.PodIP ||
		oldPod.Spec.NodeName != newPod.Spec.NodeName {
		return true
	}
	// container ready status change
	if len(oldPod.Status.ContainerStatuses) != len(newPod.Status.ContainerStatuses) {
		return true
	}
	for i := range newPod.Status.ContainerStatuses {
		if newPod.Status.ContainerStatuses[i].Ready != oldPod.Status.ContainerStatuses[i].Ready {
			return true
		}
	}
	// labels/annotations change
	if !mapEq(oldPod.Labels, newPod.Labels) || !mapEq(oldPod.Annotations, newPod.Annotations) {
		return true
	}
	return false
}

func mapEq(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func nsOrAll(ns string) string {
	if ns == "" {
		return "ALL"
	}
	return ns
}

func logf(format string, args ...any) {
	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	fmt.Printf("%s | %s\n", ts, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func exitf(format string, args ...any) {
	log.Fatalf(format, args...)
}

func blankIfEqual(a, b string) string {
	if a == b {
		return ""
	}
	return a
}