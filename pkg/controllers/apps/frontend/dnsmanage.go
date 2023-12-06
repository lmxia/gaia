package frontend

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	dns "github.com/alibabacloud-go/alidns-20150109/v2/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	util "github.com/alibabacloud-go/tea-utils/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/utils"
)

func (c *Controller) Init(accessKeyId *string, accessKeySecret *string, regionId *string) (result *dns.Client, err error) {
	config := &openapi.Config{}
	config.AccessKeyId = accessKeyId
	config.AccessKeySecret = accessKeySecret
	config.RegionId = regionId
	return dns.NewClient(config)
}

func (c *Controller) DescribeDomainRecords(client *dns.Client, domainName *string,
	frontend *v1alpha1.Frontend) (bool, error) {
	exit := common.FrontendAliyunDNSCnameNoExist
	req := &dns.DescribeDomainRecordsRequest{}
	req.DomainName = domainName
	klog.V(4).Info(tea.String("search domain name(" + tea.StringValue(domainName) + ") dns result(json)↓"))
	tryErr := func() (e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				e = r
			}
		}()
		resp, err := client.DescribeDomainRecords(req)
		if err != nil {
			return err
		}
		domainRecords := resp.Body.DomainRecords.Record
		for i, record := range domainRecords {
			klog.V(4).Infof("%s,%s", i, record)
			if *record.RR == frontend.Name {
				exit = common.FrontendAliyunDNSCnameExist
				break
			}
		}
		return nil
	}()

	if tryErr != nil {
		var error = &tea.SDKError{}
		if t, ok := tryErr.(*tea.SDKError); ok {
			error = t
		} else {
			error.Message = tea.String(tryErr.Error())
		}
		klog.Error(error.Message)
	}
	return exit, tryErr
}

func (c *Controller) AddDomainRecord(client *dns.Client, domainName *string, rr *string, recordType *string, Value *string, frontend *v1alpha1.Frontend) error {
	req := &dns.AddDomainRecordRequest{}
	req.DomainName = domainName
	req.RR = rr
	req.Type = recordType
	req.Value = Value
	tryErr := func() (e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				e = r
			}
		}()
		resp, err := client.AddDomainRecord(req)
		if err != nil {
			return err
		}

		frontend.SetLabels(map[string]string{"domainRecordId": *resp.Body.RecordId,
			"domainCname": frontend.Labels["domainCname"]})

		_, err = c.gaiaClient.AppsV1alpha1().Frontends(frontend.Namespace).Update(context.TODO(),
			frontend, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to update 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		return nil
	}()

	if tryErr != nil {
		var error = &tea.SDKError{}
		if t, ok := tryErr.(*tea.SDKError); ok {
			error = t
		} else {
			error.Message = tea.String(tryErr.Error())
		}
		klog.Error(error.Message)
	}
	return tryErr
}

func (c *Controller) DeleteDomainRecord(client *dns.Client, recordId *string) error {
	req := &dns.DeleteDomainRecordRequest{}
	req.RecordId = recordId
	tryErr := func() (e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				e = r
			}
		}()
		resp, err := client.DeleteDomainRecord(req)
		if err != nil {
			return err
		}
		klog.V(4).Info(util.ToJSONString(resp))
		return nil
	}()

	if tryErr != nil {
		var error = &tea.SDKError{}
		if t, ok := tryErr.(*tea.SDKError); ok {
			error = t
		} else {
			error.Message = tea.String(tryErr.Error())
		}
		klog.Error(error.Message)
		return error
	}
	return tryErr
}
func (c *Controller) dnsAccelerateCreate(frontend *v1alpha1.Frontend) error {
	cname := frontend.Labels["domainCname"]
	err := c.dnsAccelerateCreateDo(frontend, &cname)
	if err != nil {
		klog.Errorf("Failed to create dns accelerate of 'Frontend' %q, error == %v", frontend.Name, err)
		return err
	}
	return nil
}
func (c *Controller) dnsAccelerateCreateDo(frontend *v1alpha1.Frontend, cname *string) error {
	regionId := common.FrontendAliyunCdnRegionID
	acckey, secret := c.cdnConfig(frontend)
	client, err := c.Init(acckey, secret, &regionId)
	if err != nil {
		klog.Errorf("Failed to init dns client  'Frontend' %q, error == %v", frontend.Name, err)
		return err
	}
	for i, _ := range frontend.Spec.Cdn {
		domainName := frontend.Spec.DomainName
		RR := frontend.Name
		recordType := frontend.Spec.Cdn[i].RecordType
		value := *cname
		err = c.AddDomainRecord(client, &domainName, &RR, &recordType, &value, frontend)
		if err != nil {
			klog.Errorf("Failed to add domain record of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
	}
	return nil

}

func (c *Controller) dnsAccelerateRecycle(frontend *v1alpha1.Frontend) error {

	recordId := frontend.Labels["domainRecordId"]
	if recordId != "" {
		regionId := common.FrontendAliyunCdnRegionID
		acckey, secret := c.cdnConfig(frontend)
		client, err := c.Init(acckey, secret, &regionId)
		if err != nil {
			klog.Errorf("Failed to init dns client  'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		err = c.DeleteDomainRecord(client, &recordId)
		if err != nil {
			klog.Errorf("Failed to delete domain record of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
	}
	frontendCopy := frontend.DeepCopy()
	frontendCopy.Finalizers = utils.RemoveString(frontendCopy.Finalizers, common.FrontendAliyunFinalizers)
	_, err := c.gaiaClient.AppsV1alpha1().Frontends(frontend.Namespace).Update(context.TODO(), frontendCopy,
		metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.WarningDepth(4, fmt.Sprintf("handleFrontend: failed to remove finalizer %s "+
			"from frontend Descriptions %s: %v", common.FrontendAliyunFinalizers, frontend, err))
	}
	return nil
}

func (c *Controller) dnsAccelerateState(frontend *v1alpha1.Frontend) (bool, error) {
	regionId := common.FrontendAliyunCdnRegionID
	domainName := frontend.Spec.DomainName
	acckey, secret := c.cdnConfig(frontend)
	client, err := c.Init(acckey, secret, &regionId)
	if err != nil {
		klog.Errorf("Failed to init dns client  'Frontend' %q, error == %v", frontend.Name, err)
		return common.FrontendAliyunDNSCnameNoExist, err
	}
	exit, err := c.DescribeDomainRecords(client, &domainName, frontend)
	if err != nil {
		klog.Errorf("Failed to describe domain records 'Frontend' %q, error == %v", frontend.Name, err)
		return common.FrontendAliyunDNSCnameNoExist, err
	}
	return exit, nil
}
