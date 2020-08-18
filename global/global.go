package global

const (
	AppName    string = "notify-server"
	AppVersion string = "0.0.1"

	AppConfigFileName string = AppName + ".json"
)

type AppConfig struct {
	DebugMode       bool     `json:"debug_mode"`
	Torrent         *Torrent `json:"torrent"`
	ExcludeFileList []string `json:"exclude_files"`
	IgnoreDirChars  string   `json:"ignore_dir_chars"` // 디렉토리 이름에서 무시할 문자들
}

//@@@@@
type Torrent struct {
	DownloadPath string `json:"download_path"`
	WatchDir     string `json:"watch_dir"`
}

// @@@@@
type DirInfo struct {
	Name     string // 디렉토리 이름
	PureName string // 디렉토리 이름에서 무시할 문자들을 모두 제외한 문자열
}
