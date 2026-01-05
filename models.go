package main

// GlobalConfig 全局环境配置
type GlobalConfig struct {
	MasterConfigPath   string `mapstructure:"master_config_path"`
	RclonePath         string `mapstructure:"rclone_path"`
	TaskConfigPath     string `mapstructure:"task_config_path"`
	LogDir             string `mapstructure:"log_dir"`
	TempDir            string `mapstructure:"temp_dir"`
	DefaultVerifyLevel int    `mapstructure:"default_verify_level"`
}

// FolderPair 具体的同步文件夹对
type FolderPair struct {
	Source   string   `mapstructure:"source"`
	Dest     string   `mapstructure:"dest"`
	Includes []string `mapstructure:"includes"` // 白名单
	Excludes []string `mapstructure:"excludes"` // 黑名单
}

// ProcessMgmt 进程管理配置
type ProcessMgmt struct {
	PreKill   []string `mapstructure:"pre_kill"`
	PostStart []string `mapstructure:"post_start"`
}

// TaskConfig 任务详细配置
type TaskConfig struct {
	Name              string       `mapstructure:"name"`
	Type              string       `mapstructure:"type"`
	Folders           []FolderPair `mapstructure:"folders"`
	Realtime          bool         `mapstructure:"realtime"`
	Schedule          string       `mapstructure:"schedule"`
	ProcessManagement ProcessMgmt  `mapstructure:"process_management"`
}

// RcloneSize 用于解析 Rclone size --json 的输出
type RcloneSize struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}
