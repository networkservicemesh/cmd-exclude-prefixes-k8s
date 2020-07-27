package prefixcollector

import (
	"context"
	"github.com/pkg/errors"
	"os"
	"strings"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
	"github.com/sirupsen/logrus"
)

const (
	// ExcludedPrefixesEnv is the name of the env variable to define excluded prefixes
	ExcludedPrefixesEnv = "EXCLUDED_PREFIXES"
)

func getExcludedPrefixesChan(clientset *kubernetes.Clientset) (<-chan prefixpool.PrefixPool, error) {
	prefixes := getExcludedPrefixesFromEnv()

	// trying to get excludePrefixes from kubeadm-config, if it exists
	if configMapPrefixes, err := getExcludedPrefixesFromConfigMap(clientset); err == nil {
		poolCh := make(chan prefixpool.PrefixPool, 1)
		pool, err := prefixpool.NewPrefixPool(append(prefixes, configMapPrefixes...)...)
		if err != nil {
			return nil, err
		}
		poolCh <- pool
		return poolCh, nil
	}

	// seems like we don't have kubeadm-config in cluster, starting monitor client
	return monitorSubnets(clientset, prefixes...), nil
}

func getExcludedPrefixesFromEnv() []string {
	excludedPrefixesEnv, ok := os.LookupEnv(ExcludedPrefixesEnv)
	if !ok {
		return []string{}
	}
	logrus.Infof("Getting excludedPrefixes from ENV: %v", excludedPrefixesEnv)
	return strings.Split(excludedPrefixesEnv, ",")
}

func getExcludedPrefixesFromConfigMap(clientset *kubernetes.Clientset) ([]string, error) {
	kubeadmConfig, err := clientset.CoreV1().
		ConfigMaps("kube-system").
		Get(context.TODO(), "kubeadm-config", metav1.GetOptions{})
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	clusterConfiguration := &v1beta2.ClusterConfiguration{}
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(kubeadmConfig.Data["ClusterConfiguration"]), 4096).
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

func monitorSubnets(clientset *kubernetes.Clientset, additionalPrefixes ...string) <-chan prefixpool.PrefixPool {
	logrus.Infof("Start monitoring prefixes to exclude")
	poolCh := make(chan prefixpool.PrefixPool, 1)

	go func() {
		for {
			errCh := make(chan error)
			go monitorReservedSubnets(poolCh, errCh, clientset, additionalPrefixes)
			err := <-errCh
			logrus.Error(err)
		}
	}()

	return poolCh
}

func monitorReservedSubnets(poolCh chan prefixpool.PrefixPool, errCh chan<- error, clientset *kubernetes.Clientset, additionalPrefixes []string) {
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
		sendPrefixPool(poolCh, podSubnet, serviceSubnet, additionalPrefixes)
	}
}

func sendPrefixPool(poolCh chan prefixpool.PrefixPool, podSubnet, serviceSubnet string, additionalPrefixes []string) {
	pool, err := getPrefixPool(podSubnet, serviceSubnet, additionalPrefixes)
	if err != nil {
		logrus.Errorf("Failed to create a prefix pool: %v", err)
		return
	}
	select {
	case <-poolCh:
	default:
	}
	poolCh <- *pool
}

func getPrefixPool(podSubnet, serviceSubnet string, additionalPrefixes []string) (*prefixpool.PrefixPool, error) {
	prefixes := additionalPrefixes
	if len(podSubnet) > 0 {
		prefixes = append(prefixes, podSubnet)
	}
	if len(serviceSubnet) > 0 {
		prefixes = append(prefixes, serviceSubnet)
	}

	pool, err := prefixpool.NewPrefixPool(prefixes...)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
