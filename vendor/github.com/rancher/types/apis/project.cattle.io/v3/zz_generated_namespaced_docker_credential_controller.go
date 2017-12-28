package v3

import (
	"context"

	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	NamespacedDockerCredentialGroupVersionKind = schema.GroupVersionKind{
		Version: Version,
		Group:   GroupName,
		Kind:    "NamespacedDockerCredential",
	}
	NamespacedDockerCredentialResource = metav1.APIResource{
		Name:         "namespaceddockercredentials",
		SingularName: "namespaceddockercredential",
		Namespaced:   true,

		Kind: NamespacedDockerCredentialGroupVersionKind.Kind,
	}
)

type NamespacedDockerCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedDockerCredential
}

type NamespacedDockerCredentialHandlerFunc func(key string, obj *NamespacedDockerCredential) error

type NamespacedDockerCredentialLister interface {
	List(namespace string, selector labels.Selector) (ret []*NamespacedDockerCredential, err error)
	Get(namespace, name string) (*NamespacedDockerCredential, error)
}

type NamespacedDockerCredentialController interface {
	Informer() cache.SharedIndexInformer
	Lister() NamespacedDockerCredentialLister
	AddHandler(handler NamespacedDockerCredentialHandlerFunc)
	Enqueue(namespace, name string)
	Sync(ctx context.Context) error
	Start(ctx context.Context, threadiness int) error
}

type NamespacedDockerCredentialInterface interface {
	ObjectClient() *clientbase.ObjectClient
	Create(*NamespacedDockerCredential) (*NamespacedDockerCredential, error)
	Get(name string, opts metav1.GetOptions) (*NamespacedDockerCredential, error)
	Update(*NamespacedDockerCredential) (*NamespacedDockerCredential, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (*NamespacedDockerCredentialList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Controller() NamespacedDockerCredentialController
	AddSyncHandler(sync NamespacedDockerCredentialHandlerFunc)
	AddLifecycle(name string, lifecycle NamespacedDockerCredentialLifecycle)
}

type namespacedDockerCredentialLister struct {
	controller *namespacedDockerCredentialController
}

func (l *namespacedDockerCredentialLister) List(namespace string, selector labels.Selector) (ret []*NamespacedDockerCredential, err error) {
	err = cache.ListAllByNamespace(l.controller.Informer().GetIndexer(), namespace, selector, func(obj interface{}) {
		ret = append(ret, obj.(*NamespacedDockerCredential))
	})
	return
}

func (l *namespacedDockerCredentialLister) Get(namespace, name string) (*NamespacedDockerCredential, error) {
	var key string
	if namespace != "" {
		key = namespace + "/" + name
	} else {
		key = name
	}
	obj, exists, err := l.controller.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    NamespacedDockerCredentialGroupVersionKind.Group,
			Resource: "namespacedDockerCredential",
		}, name)
	}
	return obj.(*NamespacedDockerCredential), nil
}

type namespacedDockerCredentialController struct {
	controller.GenericController
}

func (c *namespacedDockerCredentialController) Lister() NamespacedDockerCredentialLister {
	return &namespacedDockerCredentialLister{
		controller: c,
	}
}

func (c *namespacedDockerCredentialController) AddHandler(handler NamespacedDockerCredentialHandlerFunc) {
	c.GenericController.AddHandler(func(key string) error {
		obj, exists, err := c.Informer().GetStore().GetByKey(key)
		if err != nil {
			return err
		}
		if !exists {
			return handler(key, nil)
		}
		return handler(key, obj.(*NamespacedDockerCredential))
	})
}

type namespacedDockerCredentialFactory struct {
}

func (c namespacedDockerCredentialFactory) Object() runtime.Object {
	return &NamespacedDockerCredential{}
}

func (c namespacedDockerCredentialFactory) List() runtime.Object {
	return &NamespacedDockerCredentialList{}
}

func (s *namespacedDockerCredentialClient) Controller() NamespacedDockerCredentialController {
	s.client.Lock()
	defer s.client.Unlock()

	c, ok := s.client.namespacedDockerCredentialControllers[s.ns]
	if ok {
		return c
	}

	genericController := controller.NewGenericController(NamespacedDockerCredentialGroupVersionKind.Kind+"Controller",
		s.objectClient)

	c = &namespacedDockerCredentialController{
		GenericController: genericController,
	}

	s.client.namespacedDockerCredentialControllers[s.ns] = c
	s.client.starters = append(s.client.starters, c)

	return c
}

type namespacedDockerCredentialClient struct {
	client       *Client
	ns           string
	objectClient *clientbase.ObjectClient
	controller   NamespacedDockerCredentialController
}

func (s *namespacedDockerCredentialClient) ObjectClient() *clientbase.ObjectClient {
	return s.objectClient
}

func (s *namespacedDockerCredentialClient) Create(o *NamespacedDockerCredential) (*NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Create(o)
	return obj.(*NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Get(name string, opts metav1.GetOptions) (*NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Get(name, opts)
	return obj.(*NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Update(o *NamespacedDockerCredential) (*NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Update(o.Name, o)
	return obj.(*NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) Delete(name string, options *metav1.DeleteOptions) error {
	return s.objectClient.Delete(name, options)
}

func (s *namespacedDockerCredentialClient) List(opts metav1.ListOptions) (*NamespacedDockerCredentialList, error) {
	obj, err := s.objectClient.List(opts)
	return obj.(*NamespacedDockerCredentialList), err
}

func (s *namespacedDockerCredentialClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return s.objectClient.Watch(opts)
}

// Patch applies the patch and returns the patched deployment.
func (s *namespacedDockerCredentialClient) Patch(o *NamespacedDockerCredential, data []byte, subresources ...string) (*NamespacedDockerCredential, error) {
	obj, err := s.objectClient.Patch(o.Name, o, data, subresources...)
	return obj.(*NamespacedDockerCredential), err
}

func (s *namespacedDockerCredentialClient) DeleteCollection(deleteOpts *metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return s.objectClient.DeleteCollection(deleteOpts, listOpts)
}

func (s *namespacedDockerCredentialClient) AddSyncHandler(sync NamespacedDockerCredentialHandlerFunc) {
	s.Controller().AddHandler(sync)
}

func (s *namespacedDockerCredentialClient) AddLifecycle(name string, lifecycle NamespacedDockerCredentialLifecycle) {
	sync := NewNamespacedDockerCredentialLifecycleAdapter(name, s, lifecycle)
	s.AddSyncHandler(sync)
}
