package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/internal/utils"
	"sync"
)

type EnvPrefixSource struct {
	prefixes utils.SynchronizedPrefixesContainer
}

func (e *EnvPrefixSource) Prefixes() []string {
	return e.prefixes.GetList()
}

func NewEnvPrefixSource(uncheckedPrefixes []string, notify *sync.Cond) *EnvPrefixSource {
	prefixes := utils.GetValidatedPrefixes(uncheckedPrefixes)
	source := &EnvPrefixSource{}
	source.prefixes.SetList(prefixes)
	defer notify.Broadcast()
	return source
}
