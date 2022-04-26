package main

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"

	gaiav1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/generated/clientset/versioned"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	targetClient := clientset.PlatformV1alpha1().Targets()

	target := &gaiav1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent-cluster",
		},
		Spec: gaiav1.TargetSpec{
			BootstrapToken: "2b1j4b.ys7qfar6y4fszjd9",   // 固定的
			ParentURL:      "https://192.168.1.55:6443", // 动态获取
		},
	}

	result, err := targetClient.Create(context.TODO(), target, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created target %q.\n", result.GetObjectMeta().GetName())
}
