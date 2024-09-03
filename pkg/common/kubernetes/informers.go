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

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	informerv1 "k8s.io/client-go/informers/core/v1"
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
func NewInformer(client clientset.Interface, namespaces ...string) *InformerManager {
	onceForInformer.Do(func() {
		signalHandler = setupSignalHandler()
		informerFactory = informers.NewSharedInformerFactory(client, noResyncPeriodFunc())
	})

	informerFactories := map[string]informers.SharedInformerFactory{
		defaultInformerFactoryNamespace: informerFactory,
	}

	for _, ns := range namespaces {
		informerFactories[ns] = informers.NewSharedInformerFactoryWithOptions(client, noResyncPeriodFunc(), informers.WithNamespace(ns))
	}

	return &InformerManager{
		client:                      client,
		stopCh:                      signalHandler,
		namespacedInformerFactories: informerFactories,
		namespacedSecretInformer:    make(map[string]informerv1.SecretInformer),
	}
}

// GetSecretLister creates a lister to use
func (im *InformerManager) GetSecretLister(namespace string) listerv1.SecretLister {
	return im.getSecretInformer(namespace).Lister()
}

// GetSecretInformer gets secret informer
func (im *InformerManager) GetSecretInformer(namespace string) informerv1.SecretInformer {
	return im.getSecretInformer(namespace)
}

func (im *InformerManager) getSecretInformer(namespace string) informerv1.SecretInformer {
	secretInformer, ok := im.namespacedSecretInformer[namespace]
	if ok {
		return secretInformer
	}

	factory, ok := im.namespacedInformerFactories[namespace]
	if !ok {
		factory = informers.NewSharedInformerFactoryWithOptions(im.client, noResyncPeriodFunc(), informers.WithNamespace(namespace))
		im.namespacedInformerFactories[namespace] = factory
		go factory.Start(im.stopCh)
	}

	secretInformer = factory.Core().V1().Secrets()
	im.namespacedSecretInformer[namespace] = secretInformer

	return secretInformer
}

// AddNodeListener hooks up add, update, delete callbacks
func (im *InformerManager) AddNodeListener(add, remove func(obj interface{}), update func(oldObj, newObj interface{})) {
	if im.nodeInformer == nil {
		factory, ok := im.namespacedInformerFactories[defaultInformerFactoryNamespace]
		if !ok {
			panic("no default informer factory")
		}

		im.nodeInformer = factory.Core().V1().Nodes().Informer()
	}

	im.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    add,
		UpdateFunc: update,
		DeleteFunc: remove,
	})
}

// GetNodeLister creates a lister to use
func (im *InformerManager) GetNodeLister() listerv1.NodeLister {
	factory, ok := im.namespacedInformerFactories[defaultInformerFactoryNamespace]
	if !ok {
		panic("no default informer factory")
	}
	return factory.Core().V1().Nodes().Lister()
}

// IsNodeInformerSynced returns whether node informer is synced
func (im *InformerManager) IsNodeInformerSynced() cache.InformerSynced {
	factory, ok := im.namespacedInformerFactories[defaultInformerFactoryNamespace]
	if !ok {
		panic("no default informer factory")
	}

	return factory.Core().V1().Nodes().Informer().HasSynced
}

// Listen starts the Informers. Based on client-go informer package, if the Lister has
// already been initialized, it will not re-init them. Only new non-init Listers will be initialized.
func (im *InformerManager) Listen() {
	for _, factory := range im.namespacedInformerFactories {
		go factory.Start(im.stopCh)
	}
}

// GetContext returns a context that is cancelled when the stop channel is closed
func (im *InformerManager) GetContext() context.Context {
	return wait.ContextForChannel(im.stopCh)
}
