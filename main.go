package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

var (
	GlobalCfg   GlobalConfig
	TaskList    []TaskConfig
	debounceMap sync.Map
)

func main() {
	loadConfig()
	setupLogging()

	if len(os.Args) < 2 {
		runDaemon()
		return
	}

	command := os.Args[1]
	switch command {
	case "daemon":
		runDaemon()
	case "backup", "restore":
		if len(os.Args) < 3 {
			ShowMsg("参数错误", "请指定任务名称", true)
			return
		}
		taskName := os.Args[2]
		for _, t := range TaskList {
			if t.Name == taskName {
				ExecuteTask(t, true, command)
				return
			}
		}
		ShowMsg("错误", "找不到任务: "+taskName, true)
	default:
		ShowMsg("非法指令", "未知指令: "+command, true)
	}
}

func loadConfig() {
	v := viper.New()
	v.SetConfigFile("rcloneMaster.yaml")
	if err := v.ReadInConfig(); err != nil {
		ShowMsg("启动失败", "无法找到主配置 rcloneMaster.yaml", true)
		os.Exit(1)
	}

	masterPath := v.GetString("global.master_config_path")
	if masterPath != "" {
		v.SetConfigFile(ExpandPath(masterPath))
		v.ReadInConfig()
	}
	v.UnmarshalKey("global", &GlobalCfg)

	vt := viper.New()
	vt.SetConfigFile(ExpandPath(GlobalCfg.TaskConfigPath))
	if err := vt.ReadInConfig(); err != nil {
		ShowMsg("配置错误", "无法加载任务文件: "+GlobalCfg.TaskConfigPath, true)
		os.Exit(1)
	}
	vt.UnmarshalKey("tasks", &TaskList)
}

func setupLogging() {
	logDir := ExpandPath(GlobalCfg.LogDir)
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(f)
	}
}

func runDaemon() {
	log.Println("守护进程正在后台运行...")
	c := cron.New()
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	for _, task := range TaskList {
		t := task
		if t.Schedule != "" {
			c.AddFunc(t.Schedule, func() { ExecuteTask(t, false, "backup") })
		}
		if t.Realtime {
			for _, pair := range t.Folders {
				watcher.Add(ExpandPath(pair.Source))
			}
		}
	}
	c.Start()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				for _, t := range TaskList {
					if t.Realtime {
						for _, p := range t.Folders {
							if strings.HasPrefix(event.Name, filepath.Clean(ExpandPath(p.Source))) {
								debounceExecute(t)
							}
						}
					}
				}
			}
		}
	}
}

func debounceExecute(t TaskConfig) {
	if _, loaded := debounceMap.LoadOrStore(t.Name, true); loaded {
		return
	}
	time.AfterFunc(8*time.Second, func() {
		ExecuteTask(t, false, "backup")
		debounceMap.Delete(t.Name)
	})
}
