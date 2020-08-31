package prefixcollector

import (
	"context"
	"k8s.io/client-go/kubernetes"
)

type clientSetKeyType string

// ClientSetKey is ClientSet key in context map
const ClientSetKey clientSetKeyType = "clientsetKey"

// FromContext returns ClientSet from context ctx
func FromContext(ctx context.Context) kubernetes.Interface {
	return ctx.Value(ClientSetKey).(kubernetes.Interface)
}
