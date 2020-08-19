package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

//PrintAllEnv prints all envs
func PrintAllEnv(logger logrus.FieldLogger) {
	logger.Info("All env variables:")
	for _, env := range os.Environ() {
		logger.Info(env)
	}
}
