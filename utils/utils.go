package utils

import (
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
)

func CheckErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func ToSnakeCase(str string) string {
	matchFirstRegexp := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllRegexp := regexp.MustCompile("([a-z0-9])([A-Z])")

	snakeCaseString := matchFirstRegexp.ReplaceAllString(str, "${1}_${2}")
	snakeCaseString = matchAllRegexp.ReplaceAllString(snakeCaseString, "${1}_${2}")

	return strings.ToLower(snakeCaseString)
}

func Contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
