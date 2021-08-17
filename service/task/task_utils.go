package task

import (
	"encoding/json"
	"github.com/darkkaiser/notify-server/utils"
	"strings"
)

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
		includedOneOfManyKeywords := utils.SplitExceptEmptyItems(k, "|")
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
