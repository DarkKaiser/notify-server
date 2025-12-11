package task

import "github.com/darkkaiser/notify-server/config"

// supportedTasks
type NewTaskFunc func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error)
type NewTaskResultDataFunc func() interface{}

var supportedTasks = make(map[ID]*TaskConfig)

func RegisterTask(taskID ID, config *TaskConfig) {
	supportedTasks[taskID] = config
}

type TaskConfig struct {
	CommandConfigs []*TaskCommandConfig

	NewTaskFn NewTaskFunc
}

type TaskCommandConfig struct {
	TaskCommandID CommandID

	AllowMultipleInstances bool

	NewTaskResultDataFn NewTaskResultDataFunc
}

func (c *TaskCommandConfig) equalsTaskCommandID(taskCommandID CommandID) bool {
	return c.TaskCommandID.Match(taskCommandID)
}

func findConfigFromSupportedTask(taskID ID, taskCommandID CommandID) (*TaskConfig, *TaskCommandConfig, error) {
	taskConfig, exists := supportedTasks[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.equalsTaskCommandID(taskCommandID) == true {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrCommandNotSupported
	}

	return nil, nil, ErrTaskNotSupported
}
