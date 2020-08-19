package prefixcollector

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
)

const (
	// ExcludedPrefixesEnv is the name of the env variable to define excluded prefixes
	ExcludedPrefixesEnv = "EXCLUDED_PREFIXES"
)

func getExcludedPrefixesFromKubernetesConfigFile(context context.Context) ([]string, error) {
	clientset := FromContext(context)

	kubeadmConfig, err := clientset.CoreV1().
		ConfigMaps(KubeNamespace).
		Get(context, KubeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	if configMapPrefixes, err := getExcludedPrefixesFromConfigMap(kubeadmConfig); err == nil {
		return configMapPrefixes, nil
	}
	return nil, nil
}

func getExcludedPrefixesFromConfigMap(config *v1.ConfigMap) ([]string, error) {
	clusterConfiguration := &v1beta2.ClusterConfiguration{}
	err := yaml.NewYAMLOrJSONDecoder(strings.NewReader(config.Data["ClusterConfiguration"]), 4096).
		Decode(clusterConfiguration)
	if err != nil {
		return nil, err
	}

	podSubnet := clusterConfiguration.Networking.PodSubnet
	serviceSubnet := clusterConfiguration.Networking.ServiceSubnet

	if podSubnet == "" {
		return nil, errors.New("ClusterConfiguration.Networking.PodSubnet is empty")
	}
	if serviceSubnet == "" {
		return nil, errors.New("ClusterConfiguration.Networking.ServiceSubnet is empty")
	}

	return []string{
		podSubnet,
		serviceSubnet,
	}, nil
}

func monitorSubnets(clientset *kubernetes.Clientset) <-chan []string {
	ch := make(chan []string, 1)

	go func() {
		for {
			errCh := make(chan error)
			go monitorReservedSubnets(ch, errCh, clientset)
			err := <-errCh
			logrus.Error(err)
		}
	}()

	return ch
}

func monitorReservedSubnets(ch chan []string, errCh chan<- error, clientset *kubernetes.Clientset) {
	pw, err := WatchPodCIDR(clientset)
	if err != nil {
		errCh <- err
		return
	}
	defer pw.Stop()

	sw, err := WatchServiceIpAddr(clientset)
	if err != nil {
		errCh <- err
		return
	}
	defer sw.Stop()

	var podSubnet, serviceSubnet string
	for {
		select {
		case subnet, ok := <-pw.ResultChan():
			if !ok {
				return
			}
			podSubnet = subnet.String()
		case subnet, ok := <-sw.ResultChan():
			if !ok {
				return
			}
			serviceSubnet = subnet.String()
		}
		sendPrefixes(ch, podSubnet, serviceSubnet)
	}
}

func sendPrefixes(ch chan []string, podSubnet, serviceSubnet string) {
	prefixes := getPrefixes(podSubnet, serviceSubnet)
	select {
	case <-ch:
	default:
	}
	ch <- prefixes
}

func getPrefixes(podSubnet, serviceSubnet string) []string {
	var prefixes []string
	if len(podSubnet) > 0 {
		prefixes = append(prefixes, podSubnet)
	}
	if len(serviceSubnet) > 0 {
		prefixes = append(prefixes, serviceSubnet)
	}

	return prefixes
}
