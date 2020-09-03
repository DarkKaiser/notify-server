package global

import (
	"encoding/json"
	"github.com/darkkaiser/notify-server/utils"
	"io/ioutil"
)

const (
	AppName    string = "notify-server"
	AppVersion string = "0.9.0"

	AppConfigFileName = AppName + ".json"
)

type AppConfig struct {
	Debug     bool       `json:"debug"`
	Notifiers *Notifiers `json:"notifiers"`
	Tasks     []*Task    `json:"tasks"`
}

type Notifiers struct {
	Default   string      `json:"default"`
	Telegrams []*Telegram `json:"telegram"`
}

type Telegram struct {
	Id     string `json:"id"`
	Token  string `json:"token"`
	ChatId int64  `json:"chat_id"`
}

type Task struct {
	Id       string     `json:"id"`
	Commands []*Command `json:"commands"`
}

type Command struct {
	Id         string `json:"id"`
	TimeSpec   string `json:"time_spec"`
	NotifierId string `json:"notifier_id"`
}

func InitAppConfig() *AppConfig {
	data, err := ioutil.ReadFile(AppConfigFileName)
	utils.CheckErr(err)

	var config AppConfig
	err = json.Unmarshal(data, &config)
	utils.CheckErr(err)

	return &config
}
