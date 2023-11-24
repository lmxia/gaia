package frontend

import (
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
)

func (c *Controller) VhostState(frontend *v1alpha1.Frontend) (*string, error) {
	state := `running`
	return &state, nil
}

func (c *Controller) frontendManage(frontend *v1alpha1.Frontend) error {
	//TODO download pkg from pkg server and
	//     put pkg in the pwd of vhost use

	c.frontendPkgManageNew(frontend)
	c.frontendVhostConfigManageNew(frontend)
	return nil
}

func (c *Controller) frontendPkgManageNew(frontend *v1alpha1.Frontend) (*string, error) {
	//TODO download pkg from pkg server and
	//     put pkg in the pwd of vhost use
	//pkgnam = frontend.Spec.Cdn[0].PkgName

	state := "running"
	return &state, nil
}
func (c *Controller) frontendVhostConfigManageNew(frontend *v1alpha1.Frontend) (*string, error) {
	//TODO add config to the configmap of nginx vhost pod
	state := "running"
	return &state, nil
}

func (c *Controller) frontendManageRecycle(frontend *v1alpha1.Frontend) error {
	//TODO download pkg from pkg server and
	//     put pkg in the pwd of vhost use
	c.frontendPkgManageRecycle(frontend)
	c.frontendVhostConfigManageRecycle(frontend)
	return nil
}
func (c *Controller) frontendPkgManageRecycle(frontend *v1alpha1.Frontend) (*string, error) {
	//TODO download pkg from pkg server and
	//     put pkg in the pwd of vhost use
	state := "running"
	return &state, nil
}

func (c *Controller) frontendVhostConfigManageRecycle(frontend *v1alpha1.Frontend) error {
	//TODO add config to the configmap of nginx vhost pod
	return nil
}

func (c *Controller) frontendPkgManageUpdate(frontend v1alpha1.Frontend) (*string, error) {
	//TODO download pkg from pkg server and
	//     put pkg in the pwd of vhost use
	state := "running"
	return &state, nil
}
func (c *Controller) frontendVhostConfigManageUpdate(frontend v1alpha1.Frontend) (*string, error) {
	//TODO add config to the configmap of nginx vhost pod
	state := "running"
	return &state, nil
}
