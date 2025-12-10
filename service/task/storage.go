package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

func (t *Task) dataFileName() string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", config.AppName, strutil.ToSnakeCase(string(t.GetID())), strutil.ToSnakeCase(string(t.GetCommandID())))
	return strings.ReplaceAll(filename, "_", "-")
}

func (t *Task) readTaskResultDataFromFile(v interface{}) error {
	data, err := os.ReadFile(t.dataFileName())
	if err != nil {
		// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
		var pathError *os.PathError
		if errors.As(err, &pathError) == true {
			return nil
		}

		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일을 읽는데 실패했습니다")
	}

	return json.Unmarshal(data, v)
}

func (t *Task) writeTaskResultDataToFile(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 마샬링에 실패했습니다")
	}

	if err := os.WriteFile(t.dataFileName(), data, os.FileMode(0644)); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일 쓰기에 실패했습니다")
	}

	return nil
}
