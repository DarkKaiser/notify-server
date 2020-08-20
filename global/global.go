package global

import (
	"encoding/json"
	"github.com/darkkaiser/notify-server/utils"
	"io/ioutil"
)

const (
	AppName    string = "notify-server"
	AppVersion string = "0.0.1"

	AppConfigFileName = AppName + ".json"
)

type AppConfig struct {
	Debug     bool      `json:"debug"`
	Notifiers *Notifier `json:"notifiers"`
	Tasks     []*Task   `json:"tasks"`
}

type Notifier struct {
	Telegrams []*Telegram `json:"telegram"`
}

type Telegram struct {
	Id     string `json:"id"`
	Token  string `json:"token"`
	ChatID int64  `json:"chat_id"`
}

// @@@@@
type Task struct {
	Id         string     `json:"id"`
	Commands   []*Command `json:"commands"`
	Metadata   string     `json:"metadata"`
	NotifierId string     `json:"notifierid"`
}

// @@@@@
type Command struct {
	Command string `json:"commandId"`
	Time    string `json:"time"`
}

func InitAppConfig() *AppConfig {
	data, err := ioutil.ReadFile(AppConfigFileName)
	utils.CheckErr(err)

	var appConfig AppConfig
	err = json.Unmarshal(data, &appConfig)
	utils.CheckErr(err)

	return &appConfig
}
