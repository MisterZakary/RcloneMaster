package main

import (
	"log"
	"os"
	"path/filepath" // 确保导入了 filepath
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

// 新增辅助函数：获取可执行文件所在的目录
func getExeDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "." // 报错则退回到当前目录
	}
	return filepath.Dir(ex)
}

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
	// 关键修改：获取 EXE 所在的绝对路径
	exeDir := getExeDir()
	mainConfigPath := filepath.Join(exeDir, "rcloneMaster.yaml")

	v := viper.New()
	v.SetConfigFile(mainConfigPath)
	if err := v.ReadInConfig(); err != nil {
		// 如果主配置都找不到，弹窗并退出
		ShowMsg("启动失败", "无法找到主配置: "+mainConfigPath, true)
		os.Exit(1)
	}

	// 检查重定向 (master_config_path)
	// 注意：如果重定向路径是相对路径，我们同样让它相对于 exeDir
	masterPath := v.GetString("global.master_config_path")
	if masterPath != "" {
		finalMasterPath := ExpandPath(masterPath)
		if !filepath.IsAbs(finalMasterPath) {
			finalMasterPath = filepath.Join(exeDir, finalMasterPath)
		}
		v.SetConfigFile(finalMasterPath)
		v.ReadInConfig()
	}
	v.UnmarshalKey("global", &GlobalCfg)

	// 加载任务清单 (task_config_path)
	vt := viper.New()
	finalTaskPath := ExpandPath(GlobalCfg.TaskConfigPath)
	if !filepath.IsAbs(finalTaskPath) {
		finalTaskPath = filepath.Join(exeDir, finalTaskPath)
	}

	vt.SetConfigFile(finalTaskPath)
	if err := vt.ReadInConfig(); err != nil {
		ShowMsg("配置错误", "无法加载任务文件: "+finalTaskPath, true)
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
