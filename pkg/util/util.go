package util

import (
	"context"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func GetServicesForDeploymet(ctx context.Context, deployment *apps.Deployment, client clientset.Interface) (*core.Service, error) {
	service, err := client.CoreV1().Services(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get service for deployment: %s, err: %s", deployment.Name, err)
		return nil, err
	}
	return service, nil
}
