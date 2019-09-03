package api

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.WithFields(logrus.Fields{
		"package": "api",
	})
}
