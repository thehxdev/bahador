package utils

import (
	"log"
	"os"
)

func MustBeNil(err error) {
	if err != nil {
		panic(err)
	}
}

func GetNonEmptyEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("empty environment variable: %s", key)
	}
	return v
}
