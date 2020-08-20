package prefix_sources

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"context"
	"github.com/sirupsen/logrus"
)

type KubernetesPrefixSource struct {
	prefixChan chan []string
	errorChan  chan error
}

func (kps *KubernetesPrefixSource) ErrorChan() <-chan error {
	return kps.errorChan
}

func (kps *KubernetesPrefixSource) ResultChan() <-chan []string {
	return kps.prefixChan
}

func NewKubernetesPrefixSource(context context.Context) (*KubernetesPrefixSource, error) {
	kps := &KubernetesPrefixSource{
		make(chan []string, 1),
		make(chan error),
	}

	go kps.watchSubnets(context)
	return kps, nil
}

func (kps *KubernetesPrefixSource) watchSubnets(context context.Context) {
	clientSet := prefixcollector.FromContext(context)
	for {
		go prefixcollector.MonitorReservedSubnets(kps.prefixChan, kps.errorChan, clientSet)
		err := <-kps.prefixChan
		logrus.Error(err)
	}
}
