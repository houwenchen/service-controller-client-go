package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/houwenchen/service-controller-client-go/pkg/util"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	iappsv1 "k8s.io/client-go/informers/apps/v1"
	icorev1 "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	lappsv1 "k8s.io/client-go/listers/apps/v1"
	lcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
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
	deployment := obj.(*apps.Deployment)
	klog.Infof("start add deployment: %s, in: %s", deployment.Name, deployment.Namespace)
	sc.enQueue(deployment)
}

func (sc *ServiceController) updateDeploymentFunc(oldObj interface{}, newObj interface{}) {
	oldDeployment := oldObj.(*apps.Deployment)
	curDeployment := newObj.(*apps.Deployment)
	klog.Infof("start update deployment: %s, in: %s", oldDeployment.Name, oldDeployment.Namespace)
	sc.enQueue(curDeployment)
}

func (sc *ServiceController) deleteDeploymentFunc(obj interface{}) {
	deployment := obj.(*apps.Deployment)
	klog.Infof("start delete deployment: %s, in: %s", deployment.Name, deployment.Namespace)
	sc.enQueue(deployment)
}

func (sc *ServiceController) deleteServiceFunc(obj interface{}) {
	service, ok := obj.(*core.Service)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		service, ok = tombstone.Obj.(*core.Service)
		if !ok {
			runtime.HandleError(fmt.Errorf("tombstone contained object that is not a service %#v", obj))
			return
		}
	}
	klog.Infof("start delete service: %s, in: %s", service.Name, service.Namespace)
	// 当 service 仍有 deployment 控制时，遇到删除事件，
	// 如果 deployment 的 Annotations["service"] == "true" 时，会重新创建 service
	if d := sc.getDeploymentForService(service); d != nil && d.Annotations["service"] == "true" {
		// 确保 service 已经为 0
		service, err := util.GetServicesForDeploymet(context.TODO(), d, sc.client)
		if service == nil && errors.IsNotFound(err) {
			sc.enQueue(d)
		}
	}
}

func (sc *ServiceController) getDeploymentForService(service *core.Service) *apps.Deployment {
	controllerRef := metav1.GetControllerOf(service)
	if controllerRef == nil {
		return nil
	}
	if controllerRef.Kind != apps.SchemeGroupVersion.WithKind("Deployment").Kind {
		return nil
	}
	deployment, err := sc.dLister.Deployments(service.Namespace).Get(controllerRef.Name)
	if err != nil || deployment.UID != controllerRef.UID {
		klog.Infof("can't get deployment for service: %s, in %s", service.Name, service.Namespace)
		return nil
	}
	return deployment
}

// 将需要处理的资源加入队列
func (sc *ServiceController) enQueue(deployment *apps.Deployment) {
	// 计算资源的 key
	key, err := cache.MetaNamespaceKeyFunc(deployment)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", deployment, err))
	}

	sc.queue.Add(key)
}

func (sc *ServiceController) syncDeployment(ctx context.Context, dKey string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(dKey)
	if err != nil {
		klog.Errorf("failed to split meta namespace cache key: %s, err: %s", dKey, err)
		return err
	}

	startTime := time.Now()
	klog.Infof("start syncing deployment: %s, start time: %v", name, startTime)
	defer func() {
		klog.Infof("finish syncing deployment: %s, duration time: %v", name, time.Since(startTime))
	}()

	deployment, err := sc.dLister.Deployments(namespace).Get(name)
	if errors.IsNotFound(err) {
		// 需要检查 service 是否存在，存在需要删除
		klog.Infof("deployment: %s has been deleted", name)
		return nil
	}
	if err != nil {
		return err
	}

	// 获取 deployment 的 annotation
	needService, existNeedService := deployment.Annotations["service"]
	typeService, existTypeService := deployment.Annotations["service-type"]

	// 如果没有 service 字段，不做任何 service 的处理
	if !existNeedService {
		return nil
	}

	service, err := util.GetServicesForDeploymet(ctx, deployment, sc.client)
	if !errors.IsNotFound(err) {
		return err
	}

	// 子资源控制逻辑
	// 1. 如果 service 不存在，并且需要创建 service ，则创建
	if service == nil && needService == "true" {
		svc, _ := sc.createService(deployment, typeService)
		klog.Infof("start create service: %s, in %s", name, namespace)
		_, err = sc.client.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create service: %s failed, err: %s", name, err)
			return err
		}
	}
	// 2. 如果 service 不存在，并且不需要创建 service ，则忽略
	if service == nil && needService == "false" {
		klog.Infof("needn't create service: %s, in %s", name, namespace)
		return nil
	}
	// 3. 如果 service 存在，并且不需要创建 service，则删除
	if service != nil && needService == "false" {
		klog.Infof("start delete service: %s, in %s", name, namespace)
		err := sc.client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("delete service: %s failed, err: %s", name, err)
			return err
		}
	}
	// 4. 如果 service 存在，并且需要创建 service ，
	// 4.1 如果 typeService 变化，则重建 service
	if service != nil && needService == "true" {
		// 获取 service 的类型，与 typeService 比较
		actualType := service.Spec.Type
		if existTypeService && actualType != core.ServiceType(typeService) {
			klog.Infof("start recreate service: %s, in %s", name, namespace)
			// err := sc.client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
			// if err != nil {
			// 	klog.Errorf("delete service: %s failed, err: %s", name, err)
			// 	return err
			// }
			svc, _ := sc.createService(deployment, typeService)
			// _, err = sc.client.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
			// if err != nil {
			// 	klog.Errorf("create service: %s failed, err: %s", name, err)
			// 	return err
			// }
			sc.client.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
		}
	}
	return nil
}

// 目前先适配 deloyment 中的 pod 只有一个端口的情况
// 目前只支持 ClusterIP 的情况
func (sc *ServiceController) createService(deployment *apps.Deployment, typeService string) (*core.Service, error) {
	selector := deployment.Spec.Selector.MatchLabels

	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		},
		Spec: core.ServiceSpec{
			Type:     core.ServiceType(typeService),
			Selector: selector,
			Ports: []core.ServicePort{
				{
					TargetPort: intstr.IntOrString{IntVal: deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort},
					Port:       80,
				},
			},
		},
	}

	service.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(deployment, apps.SchemeGroupVersion.WithKind("Deployment"))}

	return service, nil
}

func (sc *ServiceController) hanleError(ctx context.Context, dKey string, err error) {
	if sc.queue.NumRequeues(dKey) <= 10 {
		sc.queue.AddRateLimited(dKey)
	}

	sc.queue.Forget(dKey)
}

func (sc *ServiceController) Run(ctx context.Context) {
	defer runtime.HandleCrash()

	// 开启5个goroutine 来调用我们的worker方法
	for i := 0; i < 5; i++ {
		go wait.Until(sc.worker, time.Minute, ctx.Done())
	}
	<-ctx.Done()
}

func (sc *ServiceController) worker() {
	for sc.processNextItem() {

	}
}

func (sc *ServiceController) processNextItem() bool {
	item, shutdown := sc.queue.Get()
	if shutdown {
		return false
	}

	defer sc.queue.Done(item)

	key := item.(string)
	err := sc.syncHandler(context.TODO(), key)
	if err != nil {
		sc.hanlerError(context.TODO(), key, err)
	}
	return true
}
