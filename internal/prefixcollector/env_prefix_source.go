package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/internal/utils"
	"sync"
	"time"
)

// EnvPrefixSource is environment excluded prefixes source
type EnvPrefixSource struct {
	prefixes utils.SynchronizedPrefixesContainer
}

// Prefixes returns prefixes from source
func (e *EnvPrefixSource) Prefixes() []string {
	return e.prefixes.GetList()
}

// NewEnvPrefixSource creates EnvPrefixSource
func NewEnvPrefixSource(uncheckedPrefixes []string, notify *sync.Cond) *EnvPrefixSource {
	prefixes := utils.GetValidatedPrefixes(uncheckedPrefixes)
	source := &EnvPrefixSource{}
	source.prefixes.SetList(prefixes)
	if prefixes != nil {
		go source.notifyListeners(notify)
	}
	return source
}

func (e *EnvPrefixSource) notifyListeners(notify *sync.Cond) {
	for {
		notify.Broadcast()
		<-time.After(time.Second * 10)
	}
}
