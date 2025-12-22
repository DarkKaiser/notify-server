package task

import (
	"errors"
	"reflect"
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
