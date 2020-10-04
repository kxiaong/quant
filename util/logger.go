package util

import (
	"log"
	"os"
	"sync"
)

type logger struct {
	filename string
	*log.Logger
}

var logger *logger
var once sync.Once

func GetLogger() *logger {
	once.Do(func() {
		logger = createLogger("coinbene.log")
	})
	return logger
}

func createLogger(fname string) *logger {
	file, _ := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	return &logger{
		filename: fname,
		Logger:   log.New(file, "My app name", log.Lshortfile),
	}
}
