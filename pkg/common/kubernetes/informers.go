/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func noResyncPeriodFunc() time.Duration {
	return 0
}

var (
	signalHandler   <-chan struct{}
	informerFactory informers.SharedInformerFactory
	onceForInformer sync.Once
)

var (
	onlyOneSignalHandler = make(chan struct{})
	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

// NewInformer creates a newk8s client based on a service account
func NewInformer(client clientset.Interface, singleWatcher bool) *InformerManager {
	onceForInformer.Do(func() {
		signalHandler = setupSignalHandler()
		informerFactory = informers.NewSharedInformerFactory(client, noResyncPeriodFunc())
	})

	return &InformerManager{
		client:          client,
		stopCh:          signalHandler,
		informerFactory: informerFactory,
	}
}

// GetSecretLister creates a lister to use
func (im *InformerManager) GetSecretLister() listerv1.SecretLister {
	if im.secretInformer == nil {
		im.secretInformer = im.informerFactory.Core().V1().Secrets()
	}

	return im.secretInformer.Lister()
}

// AddNodeListener hooks up add, update, delete callbacks
func (im *InformerManager) AddNodeListener(add, remove func(obj interface{}), update func(oldObj, newObj interface{})) {
	if im.nodeInformer == nil {
		im.nodeInformer = im.informerFactory.Core().V1().Nodes().Informer()
	}

	im.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    add,
		UpdateFunc: update,
		DeleteFunc: remove,
	})
}

// Listen starts the Informers. Based on client-go informer package, if the Lister has
// already been initialized, it will not re-init them. Only new non-init Listers will be initialized.
func (im *InformerManager) Listen() {
	go im.informerFactory.Start(im.stopCh)
}

// GetNodeList gets node list
func (im *InformerManager) GetNodeList() ([]*v1.Node, error) {
	nodeLister := im.informerFactory.Core().V1().Nodes().Lister()
	return nodeLister.List(labels.Nothing())
}
