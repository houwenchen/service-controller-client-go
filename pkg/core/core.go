package core

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func NewClientSet() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		klog.Errorf("build config from ~/.kube/config failed, err: ", err)
		klog.Info("try to build config from cluster.")
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil {
			klog.Fatalf("build config from cluster failed, err: ", err)
			return nil, err
		}
		config = inClusterConfig
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("build clientSet failed, err: ", err)
		return nil, err
	}

	return clientSet, err
}
