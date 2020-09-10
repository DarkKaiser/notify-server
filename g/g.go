package g

import (
	"encoding/json"
	"github.com/darkkaiser/notify-server/utils"
	"io/ioutil"
)

const (
	AppName    string = "notify-server"
	AppVersion string = "0.1.0"

	AppConfigFileName = AppName + ".json"
)

type AppConfig struct {
	Debug     bool `json:"debug"`
	Notifiers struct {
		DefaultNotifierID string `json:"default_notifier_id"`
		Telegrams         []struct {
			ID     string `json:"id"`
			Token  string `json:"token"`
			ChatID int64  `json:"chat_id"`
		} `json:"telegrams"`
	} `json:"notifiers"`
	Tasks []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Commands []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Scheduler   struct {
				Runnable bool   `json:"runnable"`
				TimeSpec string `json:"time_spec"`
			} `json:"scheduler"`
			Notifier struct {
				Usable bool `json:"usable"`
			} `json:"notifier"`
			DefaultNotifierID string `json:"default_notifier_id"`
		} `json:"commands"`
	} `json:"tasks"`
}

func InitAppConfig() *AppConfig {
	data, err := ioutil.ReadFile(AppConfigFileName)
	utils.CheckErr(err)

	var config AppConfig
	err = json.Unmarshal(data, &config)
	utils.CheckErr(err)

	return &config
}
