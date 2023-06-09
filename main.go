package main

import (
	"context"

	"github.com/houwenchen/service-controller-client-go/pkg"
	"github.com/houwenchen/service-controller-client-go/pkg/core"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
)

func main() {
	ctx := context.Background()
	clientSet, err := core.NewClientSet()
	if err != nil {
		klog.Fatalf("create clientSet failed, err: ", err)
	}

	factory := informers.NewSharedInformerFactory(clientSet, 0)
	dInformer := factory.Apps().V1().Deployments()
	svcInformer := factory.Core().V1().Services()

	sc := pkg.NewServiceController(clientSet, dInformer, svcInformer)

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	sc.Run(ctx)
}
