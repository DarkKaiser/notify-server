package provider

import (
	"net/http"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dummyFetcher fetcher.Fetcher 인터페이스의 더미 구현체입니다.
type dummyFetcher struct{}

func (d *dummyFetcher) Close() error {
	return nil
}

func (d *dummyFetcher) Do(req *http.Request) (*http.Response, error) {
	return nil, nil
}

// TestNewTaskParams_Validation NewBase 함수 호출 시 NewTaskParams의 유효성 검증 로직을 테스트합니다.
func TestNewTaskParams_Validation(t *testing.T) {
	// 공통 테스트 데이터
	validRequest := &contract.TaskSubmitRequest{
		TaskID:     "TEST_TASK",
		CommandID:  "TEST_CMD",
		NotifierID: "telegram",
		RunBy:      contract.TaskRunByUser,
	}
	mockStorage := &mocks.MockTaskResultStore{}

	t.Run("성공: 필수 파라미터가 모두 유효한 경우", func(t *testing.T) {
		params := NewTaskParams{
			AppConfig:  &config.AppConfig{},
			Request:    validRequest,
			InstanceID: "inst_1",
			Storage:    mockStorage,
			// Scraper가 필요 없는 경우 Fetcher는 nil이어도 됨
		}

		assert.NotPanics(t, func() {
			task := NewBase(params, false)
			assert.NotNil(t, task)
			assert.Equal(t, validRequest.TaskID, task.ID())
		})
	})

	t.Run("실패: Request가 nil인 경우 패닉 발생", func(t *testing.T) {
		params := NewTaskParams{
			Request: nil, // 필수 값 누락
		}

		assert.PanicsWithValue(t, "NewBase: params.Request는 필수입니다", func() {
			NewBase(params, false)
		})
	})

	t.Run("성공: RequireScraper=true이고 Fetcher가 주입된 경우", func(t *testing.T) {
		params := NewTaskParams{
			Request: validRequest,
			Fetcher: &dummyFetcher{}, // Fetcher 주입
		}

		assert.NotPanics(t, func() {
			task := NewBase(params, true)
			require.NotNil(t, task)
			assert.NotNil(t, task.Scraper(), "Scraper가 초기화되어야 합니다")
		})
	})

	t.Run("실패: RequireScraper=true이지만 Fetcher가 nil인 경우 패닉 발생", func(t *testing.T) {
		params := NewTaskParams{
			Request: validRequest,
			Fetcher: nil, // Fetcher 누락
		}

		expectedPanicMsg := "NewBase: 스크래핑 작업에는 Fetcher 주입이 필수입니다 (TaskID=TEST_TASK)"
		assert.PanicsWithValue(t, expectedPanicMsg, func() {
			NewBase(params, true)
		})
	})

	t.Run("성공: RequireScraper=false이고 Fetcher가 nil인 경우 (Scraper 미생성)", func(t *testing.T) {
		params := NewTaskParams{
			Request: validRequest,
			Fetcher: nil,
		}

		assert.NotPanics(t, func() {
			task := NewBase(params, false)
			require.NotNil(t, task)
			assert.Nil(t, task.Scraper(), "Scraper는 생성되지 않아야 합니다")
		})
	})
}
