// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	err "errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/mcm"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileMultiClusterHub) ensureDeployment(m *operatorsv1beta1.MultiClusterHub, dep *appsv1.Deployment) (*reconcile.Result, error) {
	dplog := log.WithValues("Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

	// See if deployment already exists and create if it doesn't
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      dep.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the deployment
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			// Deployment failed
			dplog.Error(err, "Failed to create new Deployment")
			return &reconcile.Result{}, err
		}

		// Deployment was successful
		dplog.Info("Created a new Deployment")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the deployment not existing
		dplog.Error(err, "Failed to get Deployment")
		return &reconcile.Result{}, err
	}

	// Validate object based on name
	var desired *appsv1.Deployment
	var needsUpdate bool

	switch found.Name {
	case helmrepo.HelmRepoName:
		desired, needsUpdate = helmrepo.ValidateDeployment(m, r.CacheSpec, found)
	case mcm.APIServerName, mcm.ControllerName, mcm.WebhookName:
		desired, needsUpdate = mcm.ValidateDeployment(m, r.CacheSpec, found)
	default:
		dplog.Info("Could not validate deployment; unknown name")
		return nil, nil
	}

	if needsUpdate {
		err = r.client.Update(context.TODO(), desired)
		if err != nil {
			dplog.Error(err, "Failed to update Deployment.")
			return &reconcile.Result{}, err
		}
		// Spec updated - return
		return nil, nil
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureService(m *operatorsv1beta1.MultiClusterHub, s *corev1.Service) (*reconcile.Result, error) {
	svlog := log.WithValues("Service.Namespace", s.Namespace, "Service.Name", s.Name)

	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the service
		err = r.client.Create(context.TODO(), s)

		if err != nil {
			// Creation failed
			svlog.Error(err, "Failed to create new Service")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		svlog.Info("Created a new Service")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the service not existing
		svlog.Error(err, "Failed to get Service")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

// Namespace returns namespace object of given name
func (r *ReconcileMultiClusterHub) Namespace(namespace string) *unstructured.Unstructured {
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": namespace,
			},
		},
	}
	return ns
}

func (r *ReconcileMultiClusterHub) ensureSecret(m *operatorsv1beta1.MultiClusterHub, s *corev1.Secret) (*reconcile.Result, error) {
	selog := log.WithValues("Secret.Namespace", s.Namespace, "Secret.Name", s.Name)

	found := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the secret
		err = r.client.Create(context.TODO(), s)
		if err != nil {
			// Creation failed
			selog.Error(err, "Failed to create new Secret")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		selog.Info("Created a new secret")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the secret not existing
		selog.Error(err, "Failed to get Secret")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureChannel(m *operatorsv1beta1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	selog := log.WithValues("Channel.Namespace", u.GetNamespace(), "Channel.Name", u.GetName())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Channel",
		Version: "v1",
	})
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: m.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the Channel
		err = r.client.Create(context.TODO(), u)
		if err != nil {
			// Creation failed
			selog.Error(err, "Failed to create new Channel")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		selog.Info("Created a new Channel")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the Channel not existing
		selog.Error(err, "Failed to get Channel")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureSubscription(m *operatorsv1beta1.MultiClusterHub, u *unstructured.Unstructured) (*reconcile.Result, error) {
	obLog := log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Subscription",
		Version: "v1",
	})
	// Try to get API group instance
	err := r.client.Get(context.Background(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		if m.UID != "" {
			err := r.client.Create(context.Background(), u)
			if err != nil {
				// Creation failed
				obLog.Error(err, "Failed to create new instance")
				return &reconcile.Result{}, err
			}
		}

		// Creation was successful
		obLog.Info("Created new object")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Failed to get subscription")
		return &reconcile.Result{}, err
	}

	// Validate object based on type
	updated, needsUpdate := subscription.Validate(found, u)
	if needsUpdate {
		obLog.Info("Updating subscription")
		// Update the resource
		err = r.client.Update(context.TODO(), updated)
		if err != nil {
			// Update failed
			obLog.Error(err, "Failed to update object")
			return &reconcile.Result{}, err
		}
		// Spec updated - return
		return nil, nil
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) apiReady(gv schema.GroupVersion) (*reconcile.Result, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to create rest config")
		return &reconcile.Result{}, err
	}

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Error(err, "Failed to create discovery client")
		return &reconcile.Result{}, err
	}

	err = discovery.ServerSupportsVersion(c, gv)
	if err != nil {
		// Wait a little and try again
		log.Info("Waiting for API group to be available", "API group", gv)
		return &reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) copyPullSecret(m *operatorsv1beta1.MultiClusterHub, newNS string) (*reconcile.Result, error) {
	sublog := log.WithValues("Copying Secret to cert-manager namespace", m.Spec.ImagePullSecret, "Namespace.Name", utils.CertManagerNamespace)

	pullSecret := &v1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      m.Spec.ImagePullSecret,
		Namespace: m.Namespace,
	}, pullSecret)
	if err != nil {
		sublog.Error(err, "Failed to get secret")
		return &reconcile.Result{}, err
	}

	pullSecret.SetNamespace(newNS)
	pullSecret.SetSelfLink("")
	pullSecret.SetResourceVersion("")
	pullSecret.SetUID("")

	unstructuredPullSecret, err := utils.CoreToUnstructured(pullSecret)
	if err != nil {
		sublog.Error(err, "Failed to unmarshal into unstructured object")
		return &reconcile.Result{}, err
	}
	utils.AddInstallerLabel(unstructuredPullSecret, m.Name, m.Namespace)

	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      unstructuredPullSecret.GetName(),
		Namespace: newNS,
	}, unstructuredPullSecret)

	if err != nil && errors.IsNotFound(err) {
		sublog.Info(fmt.Sprintf("Creating secret %s in namespace %s", unstructuredPullSecret.GetName(), utils.CertManagerNamespace))
		err = r.client.Create(context.TODO(), unstructuredPullSecret)
		if err != nil {
			sublog.Error(err, "Failed to create secret")
			return &reconcile.Result{}, err
		}
	}
	return nil, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func readFileRaw(path string) ([]byte, error) {
	if !fileExists(path) {
		err := err.New("File" + path + "does not exist")
		log.Error(err, "File reading error")
		return nil, err
	}
	data, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Error(err, "File reading error")
		return nil, err
	}
	return data, nil
}

//ReadComponentVersionFile reads COMPONENT_VERSION file string
func (r *ReconcileMultiClusterHub) ReadComponentVersionFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error(err, "Couldn't get user home directory")
		return "", err
	}
	path := path.Join(home, "COMPONENT_VERSION")
	data, err := readFileRaw(path)
	if err != nil {
		log.Error(err, "Couldn't read component version file")
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
