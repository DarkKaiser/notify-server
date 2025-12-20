package kurly

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewTask_InvalidCommand(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()
	req := &tasksvc.SubmitRequest{
		TaskID:    ID,
		CommandID: "InvalidCommandID",
	}
	appConfig := &config.AppConfig{}

	_, err := createTask("test_instance", req, appConfig, mockFetcher)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
}
