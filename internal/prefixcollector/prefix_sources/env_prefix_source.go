package prefix_sources

import (
	"context"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

type EnvPrefixSource struct {
	prefixes []string
}

const (
	// ExcludedPrefixesEnv is the name of the env variable to define excluded prefixes
	excludedPrefixesEnv = "EXCLUDED_PREFIXES"
)

func NewEnvPrefixSource() (*EnvPrefixSource, error) {
	prefixes := []string{}
	excludedPrefixesEnv, ok := os.LookupEnv(excludedPrefixesEnv)
	if ok {
		prefixes = strings.Split(excludedPrefixesEnv, ",")
	}

	return &EnvPrefixSource{
		prefixes,
	}, nil
}

func (eps *EnvPrefixSource) getPrefixes(context context.Context) ([]string, error) {
	logrus.Infof("Getting excludedPrefixes from ENV: %v", excludedPrefixesEnv)
	return eps.prefixes, nil
}
