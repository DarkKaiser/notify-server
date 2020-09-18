package task

const (
	// TaskID
	TidLotto TaskID = "LOTTO"

	// TaskCommandID
	TcidLottoPrediction TaskCommandID = "Prediction" // 로또 번호 예측
)

type lottoPredictionData struct{}

func init() {
	supportedTasks[TidLotto] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidLottoPrediction,

			allowMultipleIntances: false,

			newTaskDataFn: func() interface{} { return &lottoPredictionData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData) taskHandler {
			if taskRunData.taskID != TidLotto {
				return nil
			}

			task := &lottoTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},
			}

			task.runFn = func(taskData interface{}) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidLottoPrediction:
					return task.runPrediction(taskData)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task
		},
	}
}

type lottoTask struct {
	task
}

func (t *lottoTask) runPrediction(taskData interface{}) (message string, changedTaskData interface{}, err error) {
	// @@@@@

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, changedTaskData, nil
}
