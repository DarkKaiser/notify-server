package task

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/darkkaiser/notify-server/pkg/strutils"
)

type equalFunc func(selem, telem interface{}) (bool, error)
type onFoundFunc func(selem, telem interface{})
type onNotFoundFunc func(selem interface{})

func takeSliceArg(x interface{}) ([]interface{}, bool) {
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

func eachSourceElementIsInTargetElementOrNot(source, target interface{}, equalFn equalFunc, onFoundFn onFoundFunc, onNotFoundFn onNotFoundFunc) error {
	if equalFn == nil {
		return errors.New("equalFn()이 할당되지 않았습니다")
	}
	sourceSlice, ok := takeSliceArg(source)
	if ok == false {
		return errors.New("source 인자의 Slice 타입 변환이 실패하였습니다")
	}
	targetSlice, ok := takeSliceArg(target)
	if ok == false {
		return errors.New("target 인자의 Slice 타입 변환이 실패하였습니다")
	}

	for _, sourceElemment := range sourceSlice {
		for _, targetElement := range targetSlice {
			equal, err := equalFn(sourceElemment, targetElement)
			if err != nil {
				return err
			}
			if equal == true {
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

func fillTaskDataFromMap(d interface{}, m map[string]interface{}) error {
	return fillTaskCommandDataFromMap(d, m)
}

func fillTaskCommandDataFromMap(d interface{}, m map[string]interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, d); err != nil {
		return err
	}
	return nil
}

func filter(s string, includedKeywords, excludedKeywords []string) bool {
	for _, k := range includedKeywords {
		includedOneOfManyKeywords := strutils.SplitAndTrim(k, "|")
		if len(includedOneOfManyKeywords) == 1 {
			if strings.Contains(s, k) == false {
				return false
			}
		} else {
			var contains = false
			for _, keyword := range includedOneOfManyKeywords {
				if strings.Contains(s, keyword) == true {
					contains = true
					break
				}
			}
			if contains == false {
				return false
			}
		}
	}

	for _, k := range excludedKeywords {
		if strings.Contains(s, k) == true {
			return false
		}
	}

	return true
}
