package task

import (
	"fmt"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
)

// TestRegistry_Concurrency Registry의 Thread-Safe 동작을 검증합니다.
func TestRegistry_Concurrency(t *testing.T) {
	t.Run("동시 등록", func(t *testing.T) {
		r := newRegistry()
		var wg sync.WaitGroup

		// 100개의 고루틴에서 동시에 다른 태스크 등록
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				taskID := ID(fmt.Sprintf("TASK_%d", index))
				r.Register(taskID, &Config{
					NewTask: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) {
						return nil, nil
					},
					Commands: []*CommandConfig{
						{
							ID:                  CommandID(fmt.Sprintf("CMD_%d", index)),
							AllowMultiple:       true,
							NewTaskResultDataFn: func() interface{} { return struct{}{} },
						},
					},
				})
			}(i)
		}

		wg.Wait()

		// 모든 태스크가 정상 등록되었는지 확인
		for i := 0; i < 100; i++ {
			taskID := ID(fmt.Sprintf("TASK_%d", i))
			cmdID := CommandID(fmt.Sprintf("CMD_%d", i))

			searchResult, err := r.findConfig(taskID, cmdID)
			assert.NoError(t, err)
			assert.NotNil(t, searchResult)
			assert.NotNil(t, searchResult.Task)
			assert.NotNil(t, searchResult.Command)
		}
	})

	t.Run("동시 등록과 조회", func(t *testing.T) {
		r := newRegistry()
		var wg sync.WaitGroup

		// 먼저 일부 태스크 등록
		for i := 0; i < 50; i++ {
			taskID := ID(fmt.Sprintf("TASK_%d", i))
			r.Register(taskID, &Config{
				NewTask: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) {
					return nil, nil
				},
				Commands: []*CommandConfig{
					{
						ID:                  CommandID(fmt.Sprintf("CMD_%d", i)),
						AllowMultiple:       true,
						NewTaskResultDataFn: func() interface{} { return struct{}{} },
					},
				},
			})
		}

		// 50개는 등록, 50개는 조회
		for i := 0; i < 100; i++ {
			wg.Add(1)
			if i < 50 {
				// 조회
				go func(index int) {
					defer wg.Done()
					taskID := ID(fmt.Sprintf("TASK_%d", index))
					cmdID := CommandID(fmt.Sprintf("CMD_%d", index))
					_, _ = r.findConfig(taskID, cmdID)
				}(i)
			} else {
				// 등록
				go func(index int) {
					defer wg.Done()
					taskID := ID(fmt.Sprintf("TASK_%d", index))
					r.Register(taskID, &Config{
						NewTask: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) {
							return nil, nil
						},
						Commands: []*CommandConfig{
							{
								ID:                  CommandID(fmt.Sprintf("CMD_%d", index)),
								AllowMultiple:       true,
								NewTaskResultDataFn: func() interface{} { return struct{}{} },
							},
						},
					})
				}(i)
			}
		}

		wg.Wait()
	})
}
