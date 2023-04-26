package pkg

import (
	"context"

	iappsv1 "k8s.io/client-go/informers/apps/v1"
	icorev1 "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	lappsv1 "k8s.io/client-go/listers/apps/v1"
	lcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ServiceController struct {
	client clientset.Interface

	syncHandler func(ctx context.Context, dKey string) error
	hanlerError func(ctx context.Context, dKey string, err error)

	// 需要监听 deployment 和 service 资源的 Lister
	dLister   lappsv1.DeploymentLister
	svcLister lcorev1.ServiceLister

	dListerSynced   cache.InformerSynced
	svcListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewServiceController(client clientset.Interface, dInformer iappsv1.DeploymentInformer, svcInformer icorev1.ServiceInformer) *ServiceController {
	sc := &ServiceController{
		client:    client,
		dLister:   dInformer.Lister(),
		svcLister: svcInformer.Lister(),
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-controller"),
	}

	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.addDeploymentFunc,
		UpdateFunc: sc.updateDeploymentFunc,
		DeleteFunc: sc.deleteDeploymentFunc,
	})
	svcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: sc.deleteServiceFunc,
	})

	sc.syncHandler = sc.syncDeployment
	sc.hanlerError = sc.hanleError

	sc.dListerSynced = dInformer.Informer().HasSynced
	sc.svcListerSynced = svcInformer.Informer().HasSynced

	return sc
}

func (sc *ServiceController) addDeploymentFunc(obj interface{}) {

}

func (sc *ServiceController) updateDeploymentFunc(oldObj interface{}, newObj interface{}) {

}

func (sc *ServiceController) deleteDeploymentFunc(obj interface{}) {

}

func (sc *ServiceController) deleteServiceFunc(obj interface{}) {

}

func (sc *ServiceController) syncDeployment(ctx context.Context, dKey string) error {
	return nil
}

func (sc *ServiceController) hanleError(ctx context.Context, dKey string, err error) {

}
