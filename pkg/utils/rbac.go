// copied from clusternet.
package utils

import (
	"context"
	"fmt"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	known "github.com/lmxia/gaia/pkg/common"
)

// EnsureClusterRole will make sure desired clusterrole exists and update it if available
func EnsureClusterRole(ctx context.Context, clusterRole v1.ClusterRole, client *kubernetes.Clientset, backoff wait.Backoff) error {
	klog.V(5).Infof("ensure ClusterRole %s...", clusterRole.Name)
	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func() (bool, error) {
		_, lastError = client.RbacV1().ClusterRoles().Create(ctx, &clusterRole, metav1.CreateOptions{})
		if lastError == nil {
			// success on the creating
			return true, nil
		}
		if !errors.IsAlreadyExists(lastError) {
			return false, nil
		}

		// try to auto update existing object
		cr, err := client.RbacV1().ClusterRoles().Get(ctx, clusterRole.Name, metav1.GetOptions{})
		if err != nil {
			lastError = err
			return false, nil
		}
		if autoUpdate, ok := cr.Annotations[known.AutoUpdateAnnotation]; ok && autoUpdate == "true" {
			_, lastError = client.RbacV1().ClusterRoles().Update(ctx, &clusterRole, metav1.UpdateOptions{})
			if lastError != nil {
				return false, nil
			}
		}
		// success on the updating
		return true, nil
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to ensure ClusterRole %s: %v", clusterRole.Name, lastError)
}

// EnsureClusterRoleBinding will make sure desired clusterrolebinding exists and update it if available
func EnsureClusterRoleBinding(ctx context.Context, clusterRolebinding v1.ClusterRoleBinding, client *kubernetes.Clientset, backoff wait.Backoff) error {
	klog.V(5).Infof("ensure ClusterRoleBinding %s...", clusterRolebinding.Name)
	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func() (bool, error) {
		_, lastError = client.RbacV1().ClusterRoleBindings().Create(ctx, &clusterRolebinding, metav1.CreateOptions{})
		if lastError == nil {
			// success on the creating
			return true, nil
		}
		if !errors.IsAlreadyExists(lastError) {
			return false, nil
		}

		// try to auto update existing object
		crb, err := client.RbacV1().ClusterRoleBindings().Get(ctx, clusterRolebinding.Name, metav1.GetOptions{})
		if err != nil {
			lastError = err
			return false, nil
		}
		if autoUpdate, ok := crb.Annotations[known.AutoUpdateAnnotation]; ok && autoUpdate == "true" {
			_, lastError = client.RbacV1().ClusterRoleBindings().Update(ctx, &clusterRolebinding, metav1.UpdateOptions{})
			if lastError != nil {
				return false, nil
			}
		}
		// success on the updating
		return true, nil
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to ensure ClusterRoleBinding %s: %v", clusterRolebinding.Name, lastError)
}

// EnsureRole will make sure desired role exists and update it if available
func EnsureRole(ctx context.Context, role v1.Role, client *kubernetes.Clientset, backoff wait.Backoff) error {
	klog.V(5).Infof("ensure Role %s...", klog.KObj(&role).String())
	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func() (bool, error) {
		_, lastError = client.RbacV1().Roles(role.Namespace).Create(ctx, &role, metav1.CreateOptions{})
		if lastError == nil {
			// success on the creating
			return true, nil
		}
		if !errors.IsAlreadyExists(lastError) {
			return false, nil
		}

		// try to auto update existing object
		r, err := client.RbacV1().Roles(role.Namespace).Get(ctx, role.Name, metav1.GetOptions{})
		if err != nil {
			lastError = err
			return false, nil
		}
		if autoUpdate, ok := r.Annotations[known.AutoUpdateAnnotation]; ok && autoUpdate == "true" {
			_, lastError = client.RbacV1().Roles(role.Namespace).Update(ctx, &role, metav1.UpdateOptions{})
			if lastError != nil {
				return false, nil
			}
		}
		// success on the updating
		return true, nil
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to ensure Role %s: %v", klog.KObj(&role).String(), lastError)
}

// EnsureRoleBinding will make sure desired rolebinding exists and update it if available
func EnsureRoleBinding(ctx context.Context, rolebinding v1.RoleBinding, client *kubernetes.Clientset, backoff wait.Backoff) error {
	klog.V(5).Infof("ensure RoleBinding %s...", klog.KObj(&rolebinding).String())
	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func() (done bool, err error) {
		_, lastError = client.RbacV1().RoleBindings(rolebinding.Namespace).Create(ctx, &rolebinding, metav1.CreateOptions{})
		if lastError == nil {
			// success on the creating
			return true, nil
		}
		if !errors.IsAlreadyExists(lastError) {
			return false, nil
		}

		// try to auto update existing object
		rb, err := client.RbacV1().RoleBindings(rolebinding.Namespace).Get(ctx, rolebinding.Name, metav1.GetOptions{})
		if err != nil {
			lastError = err
			return false, nil
		}
		if autoUpdate, ok := rb.Annotations[known.AutoUpdateAnnotation]; ok && autoUpdate == "true" {
			_, lastError = client.RbacV1().RoleBindings(rolebinding.Namespace).Update(ctx, &rolebinding, metav1.UpdateOptions{})
			if lastError != nil {
				return false, nil
			}
		}
		// success on the updating
		return true, nil
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to ensure RoleBinding %s: %v", klog.KObj(&rolebinding).String(), lastError)
}
