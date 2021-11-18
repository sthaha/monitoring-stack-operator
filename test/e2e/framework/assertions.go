package framework

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

type AssertOption struct {
	PollInterval time.Duration
	WaitTimeout  time.Duration
}

type OptionFn func(*AssertOption)

func WithTimeout(d time.Duration) OptionFn {
	return func(o *AssertOption) {
		o.WaitTimeout = d
	}
}

func WithPollInterval(d time.Duration) OptionFn {
	return func(o *AssertOption) {
		o.PollInterval = d
	}
}

// AssertResourceNeverExists asserts that a statefulset is never created for the duration of wait.ForeverTestTimeout
func (f *Framework) AssertResourceNeverExists(name, namespace string, resource client.Object, fns ...OptionFn) func(t *testing.T) {
	option := AssertOption{
		PollInterval: 5 * time.Second,
		WaitTimeout:  wait.ForeverTestTimeout,
	}
	for _, fn := range fns {
		fn(&option)
	}

	return func(t *testing.T) {
		if err := wait.Poll(5*time.Second, wait.ForeverTestTimeout, func() (done bool, err error) {
			key := types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}
			if err := f.K8sClient.Get(context.Background(), key, resource); errors.IsNotFound(err) {
				return false, nil
			}

			return true, fmt.Errorf("statefulset %s/%s should not have been created", namespace, name)
		}); err != wait.ErrWaitTimeout {
			t.Fatal(err)
		}
	}
}

// AssertResourceEventuallyExists asserts that a statefulset is created duration a time period of wait.ForeverTestTimeout
func (f *Framework) AssertResourceEventuallyExists(name, namespace string, resource client.Object, fns ...OptionFn) func(t *testing.T) {
	option := AssertOption{
		PollInterval: 5 * time.Second,
		WaitTimeout:  wait.ForeverTestTimeout,
	}
	for _, fn := range fns {
		fn(&option)
	}

	return func(t *testing.T) {
		if err := wait.Poll(option.PollInterval, option.WaitTimeout, func() (done bool, err error) {
			key := types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}
			if err := f.K8sClient.Get(context.Background(), key, resource); err == nil {
				return true, nil
			}
			return false, nil
		}); err == wait.ErrWaitTimeout {
			t.Fatal(fmt.Errorf("statefulset %s/%s was never created", namespace, name))
		}
	}
}

// AssertPodEventuallyRuns asserts that a pod eventually gets into a Running phase
func (f *Framework) AssertPodEventuallyRuns(name string, namespace string) func(t *testing.T) {
	return func(t *testing.T) {
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if err := wait.Poll(5*time.Second, wait.ForeverTestTimeout, func() (bool, error) {
			pod := &corev1.Pod{}
			err := f.K8sClient.Get(context.Background(), key, pod)
			return err == nil && pod.Status.Phase == corev1.PodRunning, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *Framework) GetResourceWithRetry(t *testing.T, name, namespace string, obj client.Object) {
	err := wait.Poll(5*time.Second, wait.ForeverTestTimeout, func() (bool, error) {
		key := types.NamespacedName{Name: name, Namespace: namespace}

		if err := f.K8sClient.Get(context.Background(), key, obj); errors.IsNotFound(err) {
			// retry
			return false, nil
		}

		return true, nil
	})

	if err == wait.ErrWaitTimeout {
		t.Fatal(fmt.Errorf("resource %s/%s was never created", namespace, name))
	}
}
