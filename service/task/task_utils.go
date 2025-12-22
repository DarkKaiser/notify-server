package task

import (
	"errors"
	"reflect"
	"strings"

	"github.com/darkkaiser/notify-server/pkg/strutil"
)

type EqualFunc func(selem, telem interface{}) (bool, error)
type OnFoundFunc func(selem, telem interface{})
type OnNotFoundFunc func(selem interface{})

func TakeSliceArg(x interface{}) ([]interface{}, bool) {
	value := reflect.ValueOf(x)
	if value.Kind() != reflect.Slice {
		return nil, false
	}

	result := make([]interface{}, value.Len())
	for index := 0; index < value.Len(); index++ {
		result[index] = value.Index(index).Interface()
	}

	return result, true
}

func EachSourceElementIsInTargetElementOrNot(source, target interface{}, equalFn EqualFunc, onFoundFn OnFoundFunc, onNotFoundFn OnNotFoundFunc) error {
	if equalFn == nil {
		return errors.New("equalFn()이 할당되지 않았습니다")
	}
	sourceSlice, ok := TakeSliceArg(source)
	if !ok {
		return errors.New("source 인자의 Slice 타입 변환이 실패하였습니다")
	}
	targetSlice, ok := TakeSliceArg(target)
	if !ok {
		return errors.New("target 인자의 Slice 타입 변환이 실패하였습니다")
	}

	for _, sourceElemment := range sourceSlice {
		for _, targetElement := range targetSlice {
			equal, err := equalFn(sourceElemment, targetElement)
			if err != nil {
				return err
			}
			if equal {
				if onFoundFn != nil {
					onFoundFn(sourceElemment, targetElement)
				}
				goto NEXTITEM
			}
		}

		if onNotFoundFn != nil {
			onNotFoundFn(sourceElemment)
		}

	NEXTITEM:
	}

	return nil
}

func Filter(s string, includedKeywords, excludedKeywords []string) bool {
	// 대소문자 구분 없이 비교하기 위해 소문자로 변환
	lowerS := strings.ToLower(s)

	for _, k := range includedKeywords {
		includedOneOfManyKeywords := strutil.SplitAndTrim(k, "|")
		if len(includedOneOfManyKeywords) == 1 {
			lowerK := strings.ToLower(k)
			if !strings.Contains(lowerS, lowerK) {
				return false
			}
		} else {
			var contains = false
			for _, keyword := range includedOneOfManyKeywords {
				if strings.Contains(lowerS, strings.ToLower(keyword)) {
					contains = true
					break
				}
			}
			if !contains {
				return false
			}
		}
	}

	for _, k := range excludedKeywords {
		if strings.Contains(lowerS, strings.ToLower(k)) {
			return false
		}
	}

	return true
}
