package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplication(t *testing.T) {
	t.Run("Application 구조체 생성", func(t *testing.T) {
		app := &Application{
			ID:                "test-app",
			Title:             "Test Application",
			Description:       "Test Description",
			DefaultNotifierID: "test-notifier",
			AppKey:            "secret-key-123",
		}

		assert.Equal(t, "test-app", app.ID, "ID가 일치해야 합니다")
		assert.Equal(t, "Test Application", app.Title, "Title이 일치해야 합니다")
		assert.Equal(t, "Test Description", app.Description, "Description이 일치해야 합니다")
		assert.Equal(t, "test-notifier", app.DefaultNotifierID, "DefaultNotifierID가 일치해야 합니다")
		assert.Equal(t, "secret-key-123", app.AppKey, "AppKey가 일치해야 합니다")
	})

	t.Run("빈 Application", func(t *testing.T) {
		app := &Application{}

		assert.Empty(t, app.ID, "ID가 비어있어야 합니다")
		assert.Empty(t, app.Title, "Title이 비어있어야 합니다")
		assert.Empty(t, app.Description, "Description이 비어있어야 합니다")
		assert.Empty(t, app.DefaultNotifierID, "DefaultNotifierID가 비어있어야 합니다")
		assert.Empty(t, app.AppKey, "AppKey가 비어있어야 합니다")
	})
}
