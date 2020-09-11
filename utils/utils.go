package utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strings"
)

func CheckErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CheckStatusCode(res *http.Response) {
	if res.StatusCode != 200 {
		log.Fatal("Request failed with Status:", res.StatusCode)
	}
}

func CleanString(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func FormatCommas(n int) string {
	str := fmt.Sprintf("%d", n)
	re := regexp.MustCompile("(\\d+)(\\d{3})")
	for n := ""; n != str; {
		n = str
		str = re.ReplaceAllString(str, "$1,$2")
	}
	return str
}

func ToSnakeCase(str string, separator string) string {
	matchFirstRegexp := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllRegexp := regexp.MustCompile("([a-z0-9])([A-Z])")

	repl := fmt.Sprintf("${1}%s${2}", separator)
	snakeCaseString := matchFirstRegexp.ReplaceAllString(str, repl)
	snakeCaseString = matchAllRegexp.ReplaceAllString(snakeCaseString, repl)

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
