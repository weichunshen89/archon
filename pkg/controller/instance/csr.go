/*
Copyright 2016 The Archon Authors.
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

package instance

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/v1"
	"kubeup.com/archon/pkg/clientset"
	"kubeup.com/archon/pkg/cluster"
	"kubeup.com/archon/pkg/controller/certificate"
	"kubeup.com/archon/pkg/initializer"

	"fmt"
)

var (
	CSRToken = cluster.AnnotationPrefix + "csr"
)

type CSRInitializer struct {
	kubeClient clientset.Interface
}

var _ initializer.Initializer = &CSRInitializer{}

func NewCSRInitializer(kubeClient clientset.Interface) (initializer.Initializer, error) {

	c := &CSRInitializer{
		kubeClient: kubeClient,
	}
	return c, nil
}

func (ci *CSRInitializer) Token() string {
	return CSRToken
}

func (ci *CSRInitializer) Initialize(obj initializer.Object) (updatedObj initializer.Object, err error, retryable bool) {
	instance, ok := obj.(*cluster.Instance)
	if !ok {
		err = fmt.Errorf("expecting Instance. got %v", obj)
		return
	}

	if initializer.HasInitializer(instance, PublicIPToken, PrivateIPToken) {
		err = initializer.ErrSkip
		return
	}

	var secret *v1.Secret
	notReady := 0
	for _, n := range instance.Spec.Secrets {
		secret, err = ci.kubeClient.Core().Secrets(instance.Namespace).Get(n.Name, metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("Failed to get secret resource %s: %v", n.Name, err)
			return
		}
		if status, ok := secret.Annotations[certificate.ResourceStatusKey]; ok {
			if status != "Ready" {
				if status == "Pending" {
					secret.Annotations[certificate.ResourceStatusKey] = "Approved"
					_, err = ci.kubeClient.Core().Secrets(instance.Namespace).Update(secret)
					if err != nil {
						err = fmt.Errorf("Failed to generate certificate %s: %v", n.Name, err)
						return
					}
				}
				notReady += 1
			}
		}
	}

	if notReady > 0 {
		err = initializer.ErrSkip
		return
	}

	initializer.RemoveInitializer(instance, CSRToken)
	updatedObj, err = ci.kubeClient.Archon().Instances(instance.Namespace).Update(instance)
	if err != nil {
		retryable = true
	}
	return
}

func (ci *CSRInitializer) Finalize(obj initializer.Object) (updatedObj initializer.Object, err error, retryable bool) {
	return
}
