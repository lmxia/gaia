package frontend

import (
	"context"
	"encoding/base64"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	cdn20180510 "github.com/alibabacloud-go/cdn-20180510/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	str "github.com/alibabacloud-go/darabonba-string/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
)

func (c *Controller) cdnConfig(frontend *v1alpha1.Frontend) (*string, *string, error) {
	cdnSupplierMsg, err := c.cdnSupplierLister.CdnSuppliers(common.GaiaFrontendNamespace).
		Get(frontend.Spec.Cdn[0].Supplier)
	if err != nil {
		klog.Errorf("Failed to get cdn supplier of 'Frontend' %q, error == %v", frontend.Name, err)
		return nil, nil, err
	}
	// RSA
	secertKey := cdnSupplierMsg.Spec.CloudAccessKeyid
	secertID := cdnSupplierMsg.Spec.CloudAccessKeysecret
	return &secertKey, &secertID, nil
}

func (c *Controller) supplierClient(frontend *v1alpha1.Frontend) (result *cdn20180510.Client, err error) {
	acckey, secret, err2 := c.cdnConfig(frontend)
	if err2 != nil {
		return nil, err2
	}
	config := &openapi.Config{
		AccessKeyId:     acckey,
		AccessKeySecret: secret,
	}
	config.Endpoint = tea.String(common.FrontendAliyunCdnEndpoint)
	return cdn20180510.NewClient(config)
}

func (c *Controller) cdnAccelerateState(frontend *v1alpha1.Frontend) (bool, *string, error) {
	client, err := c.supplierClient(frontend)
	if err != nil {
		return false, nil, err
	}
	var domainStatus string
	exist := common.FrontendAliyunCdnNoExist
	describeUserDomainsRequest := &cdn20180510.DescribeUserDomainsRequest{}
	runtime := &util.RuntimeOptions{}
	tryErr := func() (e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				e = r
			}
		}()
		domainMsg, domainErr := client.DescribeUserDomainsWithOptions(describeUserDomainsRequest, runtime)
		if domainErr != nil {
			klog.Error(domainErr)
			return domainErr
		}
		for i := range domainMsg.Body.Domains.PageData {
			cname := domainMsg.Body.Domains.PageData[i].Cname
			domainName := domainMsg.Body.Domains.PageData[i].DomainName
			klog.V(4).Infof("cname and domainName of frontend is: %s", cname, domainName)
			frontendDomain := frontend.Name + "." + frontend.Spec.DomainName
			if frontendDomain == *domainMsg.Body.Domains.PageData[i].DomainName {
				exist = common.FrontendAliyunCdnExist
				domainStatus = *domainMsg.Body.Domains.PageData[i].DomainStatus
				break
			}
		}
		return nil
	}()

	if tryErr != nil {
		sdkError := &tea.SDKError{}
		if t, ok := tryErr.(*tea.SDKError); ok {
			sdkError = t
		} else {
			sdkError.Message = tea.String(tryErr.Error())
		}
		_, err = util.AssertAsString(sdkError.Message)
		if err != nil {
			return false, nil, err
		}
	}
	return exist, &domainStatus, err
}

func (c *Controller) cdnAccelerateCreate(frontend *v1alpha1.Frontend, cdnStatus *string) error {
	domainName := frontend.Name + "." + frontend.Spec.DomainName

	for i, supplierName := range frontend.Spec.Cdn {
		if supplierName.Supplier == common.FrontendAliyunCdnName {
			cdnType := frontend.Spec.Cdn[i].CdnType
			frontend.Spec.Cdn[i].SourceSite = c.aliyunSourceSite
			sources := frontend.Spec.Cdn[i].SourceSite
			scope := frontend.Spec.Cdn[i].AccerlateRegion
			client, err := c.supplierClient(frontend)
			if err != nil {
				klog.Errorf("Failed to new cdn client  of 'Frontend' %q, error == %v", frontend.Name, err)
				return err
			}
			err = c.processCdnAccelerateCreate(client, &domainName, &cdnType, &sources, &scope, frontend, cdnStatus)
			if err != nil {
				klog.Errorf("Failed to add cdn accelerate of 'Frontend' %q, error == %v", frontend.Name, err)
				return err
			}
		}
	}
	return nil
}

func (c *Controller) processCdnAccelerateCreate(client *cdn20180510.Client, domain *string, cdnType *string,
	sources *string, scope *string, frontend *v1alpha1.Frontend, cdnStatus *string,
) error {
	tagDomainName := &cdn20180510.AddCdnDomainRequestTag{
		Key:   tea.String(common.FrontendAliyunCdnTagDomainName),
		Value: tea.String(base64.StdEncoding.EncodeToString([]byte((*domain)))),
	}
	tagUserID := &cdn20180510.AddCdnDomainRequestTag{
		Key:   tea.String(common.FrontendAliyunCdnTagUserID),
		Value: tea.String(base64.StdEncoding.EncodeToString([]byte((frontend.Labels[common.UserNameLabel])))),
	}
	tagComponentName := &cdn20180510.AddCdnDomainRequestTag{
		Key:   tea.String(common.FrontendAliyunCdnTagComponentName),
		Value: tea.String(base64.StdEncoding.EncodeToString([]byte((frontend.Labels[common.GaiaComponentLabel])))),
	}
	tagSupplierName := &cdn20180510.AddCdnDomainRequestTag{
		Key:   tea.String(common.FrontendAliyunCdnTagSupplierName),
		Value: tea.String(base64.StdEncoding.EncodeToString([]byte((frontend.Spec.Cdn[0].Supplier)))),
	}
	request := &cdn20180510.AddCdnDomainRequest{
		Tag:        []*cdn20180510.AddCdnDomainRequestTag{tagDomainName, tagUserID, tagComponentName, tagSupplierName},
		DomainName: domain,
		CdnType:    cdnType,
		Sources:    sources,
		Scope:      scope,
	}
	tryErr := func() (e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				e = r
			}
		}()
		var domainName string
		if *cdnStatus != common.FrontendAliyunCdnConfigureStatus {
			response, err := client.AddCdnDomain(request)
			if err != nil {
				klog.Errorf("Failed to add cdn domain of 'Frontend' %q, error == %v", frontend.Name, err)
				return err
			}
			klog.V(4).Infof("add domain succeed：" + tea.StringValue(response.Body.RequestId))
			domainName = tea.StringValue(domain)
		}
		for {
			status, cname, err := c.GetDomainStatus(client, &domainName)
			if err != nil {
				return err
			}
			if tea.BoolValue(util.EqualString(status, tea.String(common.FrontendAliyunCdnOnlineStatus))) {
				err = util.Sleep(tea.Int(common.FrontendAliyunCdnSleepWait))
				if err != nil {
					return err
				}
				if frontend.Labels == nil {
					frontend.SetLabels(map[string]string{"domainCname": *cname})
				} else {
					frontend.GetLabels()["domainCname"] = *cname
				}
				frontendUpdate, errUpdate := c.gaiaClient.AppsV1alpha1().Frontends(frontend.Namespace).Update(context.TODO(),
					frontend, metav1.UpdateOptions{})
				if errUpdate != nil {
					klog.Errorf("Failed to update  'Frontend' %q, error == %v", frontend.Name, errUpdate)
					return errUpdate
				}
				frontend.SetResourceVersion(frontendUpdate.ResourceVersion)
				err = c.dnsAccelerateCreateDo(frontend, cname)
				if err != nil {
					klog.Errorf("Failed to add cdn accelerate of 'Frontend' %q, error == %v", frontend.Name, err)
					return err
				}
				break
			}
			err = util.Sleep(tea.Int(common.FrontendAliyunCdnSleepError))
			if err != nil {
				klog.Error(err)
				return err
			}
		}
		return nil
	}()
	if tryErr != nil {
		error := &tea.SDKError{}
		if t, ok := tryErr.(*tea.SDKError); ok {
			error = t
		} else {
			error.Message = tea.String(tryErr.Error())
		}
		klog.Error(error.Message)
		return tryErr
	}
	return nil
}

func (c *Controller) GetDomainStatus(client *cdn20180510.Client, domain *string) (result *string,
	cname *string, err error,
) {
	request := &cdn20180510.DescribeUserDomainsRequest{
		DomainName: domain,
	}
	response, err := client.DescribeUserDomains(request)
	if err != nil {
		klog.Errorf("Failed to describe user domains 'Cname' %q, error == %v", cname, err)
		return result, cname, err
	}
	pageData := response.Body.Domains.PageData[0]
	klog.V(4).Infof("get the name of domain  is " + tea.StringValue(domain) + "and the status of domain is " +
		tea.StringValue(pageData.DomainStatus))
	result = pageData.DomainStatus
	cname = pageData.Cname
	return result, cname, err
}

func (c *Controller) cdnAccelerateUpdate(frontend *v1alpha1.Frontend, cdnStatus *string) error {
	// configure dns by current cdn state
	switch *cdnStatus {
	case common.FrontendAliyunCdnOnlineStatus:
		// online
		dnsAccelerateState, err := c.dnsAccelerateState(frontend)
		if err != nil {
			klog.Errorf("Failed to get dns AccelerateState of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		if !dnsAccelerateState {
			cname := frontend.Labels["domainCname"]
			err := c.dnsAccelerateCreateDo(frontend, &cname)
			if err != nil {
				klog.Errorf("Failed to do dns accelerate create of 'Frontend' %q, error == %v", frontend.Name, err)
				return err
			}
		}
	case common.FrontendAliyunCdnConfigureStatus:
		// configure
		err := c.cdnAccelerateCreate(frontend, cdnStatus)
		if err != nil {
			klog.Errorf("Failed to do cdn Accelerate create of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
	}
	return nil
}

func (c *Controller) cdnAccelerateRecycle(frontend *v1alpha1.Frontend) error {
	domainName := frontend.Name + "." + frontend.Spec.DomainName
	for _, supplierName := range frontend.Spec.Cdn {
		if supplierName.Supplier == common.FrontendAliyunCdnName {
			client, err := c.supplierClient(frontend)
			if err != nil {
				return err
			}
			tryErr := func() (e error) {
				defer func() {
					if r := tea.Recover(recover()); r != nil {
						e = r
					}
				}()
				domains := tea.String(tea.StringValue(&domainName))
				domainArray := str.Split(domains, tea.String(","), tea.Int(3))
				for _, domain := range domainArray {
					for {
						status, _, errGetDomain := c.GetDomainStatus(client, domain)
						if errGetDomain != nil {
							return errGetDomain
						}
						if tea.BoolValue(util.EqualString(status, tea.String(common.FrontendAliyunCdnOnlineStatus))) {
							if err != nil {
								return err
							}
							err = c.DeleteDomain(client, domain)
							if err != nil {
								return err
							}
							break
						}
						err = util.Sleep(tea.Int(common.FrontendAliyunCdnSleepError))
						if err != nil {
							return err
						}
					}
				}
				return nil
			}()
			if tryErr != nil {
				error := &tea.SDKError{}
				if t, ok := tryErr.(*tea.SDKError); ok {
					error = t
				} else {
					error.Message = tea.String(tryErr.Error())
				}
				klog.Error(error.Message)
			}
			return err
		}
	}
	return nil
}

func (c *Controller) DeleteDomain(client *cdn20180510.Client, domain *string) (err error) {
	request := &cdn20180510.DeleteCdnDomainRequest{
		DomainName: domain,
	}
	response, err := client.DeleteCdnDomain(request)
	if err != nil {
		klog.Error(err)
		return err
	}
	klog.V(4).Infof("succeed in delete domain: " + tea.StringValue(response.Body.RequestId))
	return err
}
