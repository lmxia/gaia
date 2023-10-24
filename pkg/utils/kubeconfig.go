// copied from clusternet.
package utils

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// createBasicKubeConfig creates a basic, general KubeConfig object that then can be extended
func createBasicKubeConfig(serverURL, clusterName, userName string, caCert []byte) *clientcmdapi.Config {
	// Use the cluster and the username as the context name
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	var insecureSkipTLSVerify bool
	if caCert == nil {
		insecureSkipTLSVerify = true
	}

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				InsecureSkipTLSVerify:    insecureSkipTLSVerify,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
}

// CreateKubeConfigWithToken creates a KubeConfig object with access to the API server with a token
func CreateKubeConfigWithToken(serverURL, token string, caCert []byte) *clientcmdapi.Config {
	userName := "gaia"
	clusterName := "gaia-cluster"
	config := createBasicKubeConfig(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Token: token,
	}
	return config
}

// LoadsKubeConfig tries to load kubeconfig from specified kubeconfig file or in-cluster config
func LoadsKubeConfig(kubeConfigPath string, flowRate int) (*rest.Config, error) {
	if len(kubeConfigPath) == 0 {
		// use in-cluster config
		return rest.InClusterConfig()
	}

	clientConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error while loading kubeconfig from file %v: %v", kubeConfigPath, err)
	}
	config, err := clientcmd.NewDefaultClientConfig(*clientConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}
	return applyDefaultRateLimiter(config, flowRate), nil
}

// GenerateKubeConfigFromToken composes a kubeconfig from token
func GenerateKubeConfigFromToken(serverURL, token string, caCert []byte, flowRate int) (*rest.Config, error) {
	clientConfig := CreateKubeConfigWithToken(serverURL, token, caCert)
	config, err := clientcmd.NewDefaultClientConfig(*clientConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}

	return applyDefaultRateLimiter(config, flowRate), nil
}

func applyDefaultRateLimiter(config *rest.Config, flowRate int) *rest.Config {
	if flowRate < 0 {
		flowRate = 1
	}

	// here we magnify the default qps and burst in client-go
	config.QPS = rest.DefaultQPS * float32(flowRate)
	config.Burst = rest.DefaultBurst * flowRate

	return config
}
