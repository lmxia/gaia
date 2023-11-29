package frontend

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
)

func (c *Controller) frontendManage(frontend *v1alpha1.Frontend) error {
	vhost, err := c.vhostClient.FrontendsV1().Vhosts(frontend.Namespace).Get(context.TODO(), frontend.Name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Errorf("Failed to get vhost  record of 'Frontend' %q, error == %v", frontend.Name, err)
			vhost.Kind = common.FrontendAliyunCdnVhostKind
			vhost.APIVersion = common.FrontendAliyunCdnVhostAPIVersion
			vhost.Finalizers = append(vhost.Finalizers, common.FrontendAliyunCdnVhostFinalizer)
			vhost.Name = frontend.Name
			vhost.Namespace = frontend.Namespace
			vhost.Spec.DomainName = frontend.Spec.DomainName
			vhost.Spec.PkgName = frontend.Spec.Cdn[0].PkgName
			_, err = c.vhostClient.FrontendsV1().Vhosts(frontend.Namespace).Create(context.TODO(), vhost, v1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create vhost  record of 'Frontend' %q, error == %v", frontend.Name, err)
			}
			klog.V(4).Infof("succeed in create vhost of Frontend' %q,  vhost msg is:%s"+frontend.Name, vhost)
			return err
		}
	}
	return nil
}

func (c *Controller) frontendManageRecycle(frontend *v1alpha1.Frontend) error {
	err := c.vhostClient.FrontendsV1().Vhosts(frontend.Namespace).Delete(context.TODO(), frontend.Name, v1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Failed to recycle vhost  record of 'Frontend' %q, error == %v", frontend.Name, err)
		return err
	}
	return nil
}
