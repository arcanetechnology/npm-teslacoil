package main

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.WithFields(logrus.Fields{
		"package": "lpp",
	})
}
