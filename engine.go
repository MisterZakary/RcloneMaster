package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// ExpandPath 支持 Windows 环境变量如 %APPDATA%
func ExpandPath(path string) string {
	re := regexp.MustCompile(`%([^%]+)%`)
	return re.ReplaceAllStringFunc(path, func(s string) string {
		v := strings.Trim(s, "%")
		return os.Getenv(v)
	})
}

// ShowMsg 弹出 Windows 消息框
func ShowMsg(title, msg string, isError bool) {
	tPtr, _ := windows.UTF16PtrFromString(title)
	mPtr, _ := windows.UTF16PtrFromString(msg)
	var icon uint32 = windows.MB_OK
	if isError {
		icon |= windows.MB_ICONERROR
	} else {
		icon |= windows.MB_ICONINFORMATION
	}
	windows.MessageBox(0, mPtr, tPtr, icon)
}

// killProcess 具有重试逻辑的进程杀死功能
func killProcess(name string) bool {
	for i := 1; i <= 5; i++ {
		check := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name)
		check.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		out, _ := check.Output()
		if !strings.Contains(string(out), name) {
			return true
		}

		args := []string{"/IM", name, "/T"}
		if i > 2 {
			args = append(args, "/F")
		}
		cmd := exec.Command("taskkill", args...)
		cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		cmd.Run()
		time.Sleep(1 * time.Second)
	}
	return false
}

// runRclone 执行同步命令
func runRclone(taskType, src, dst string, pair FolderPair, isManual bool) error {
	args := []string{strings.ToLower(taskType), ExpandPath(src), ExpandPath(dst)}
	if len(pair.Includes) > 0 {
		for _, inc := range pair.Includes {
			args = append(args, "--include", inc)
		}
	} else if len(pair.Excludes) > 0 {
		for _, ex := range pair.Excludes {
			args = append(args, "--exclude", ex)
		}
	}
	args = append(args, "--ignore-errors", "--buffer-size", "32M")
	cmd := exec.Command(ExpandPath(GlobalCfg.RclonePath), args...)
	cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

// verify 校验数据一致性（带过滤规则）
func verify(src, dst string, pair FolderPair) error {
	if GlobalCfg.DefaultVerifyLevel == 0 {
		return nil
	}
	getSize := func(p string) (RcloneSize, error) {
		args := []string{"size", ExpandPath(p), "--json"}
		if len(pair.Includes) > 0 {
			for _, inc := range pair.Includes {
				args = append(args, "--include", inc)
			}
		} else if len(pair.Excludes) > 0 {
			for _, ex := range pair.Excludes {
				args = append(args, "--exclude", ex)
			}
		}
		c := exec.Command(ExpandPath(GlobalCfg.RclonePath), args...)
		c.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		out, err := c.Output()
		if err != nil {
			return RcloneSize{}, err
		}
		var rs RcloneSize
		err = json.Unmarshal(out, &rs)
		return rs, err
	}
	sSize, _ := getSize(src)
	dSize, _ := getSize(dst)
	if sSize.Bytes != dSize.Bytes || sSize.Count != dSize.Count {
		return fmt.Errorf("校验失败(带过滤): 源 %d文件/%d字节, 目标 %d文件/%d字节",
			sSize.Count, sSize.Bytes, dSize.Count, dSize.Bytes)
	}
	return nil
}

// ExecuteTask 任务执行流水线
func ExecuteTask(task TaskConfig, isManual bool, mode string) {
	tempDir := ExpandPath(GlobalCfg.TempDir)
	os.MkdirAll(tempDir, 0755)
	lockFile := filepath.Join(tempDir, task.Name+".lock")

	if _, err := os.Stat(lockFile); err == nil {
		if isManual {
			ShowMsg("任务跳过", "任务 ["+task.Name+"] 已经在运行中。", false)
		}
		return
	}
	os.WriteFile(lockFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	defer os.Remove(lockFile)

	log.Printf("[%s] 开始执行, 模式: %s", task.Name, mode)

	// 1. Pre-Kill
	if len(task.ProcessManagement.PreKill) > 0 {
		for _, p := range task.ProcessManagement.PreKill {
			if !killProcess(p) {
				handleError(task.Name, "无法关闭进程: "+p, isManual)
				return
			}
		}
		time.Sleep(1200 * time.Millisecond) // 给 Everything 刷写配置留一点时间
	}

	// 2. 文件夹循环同步
	for i, pair := range task.Folders {
		src, dst := pair.Source, pair.Dest
		if mode == "restore" {
			src, dst = pair.Dest, pair.Source
		}

		log.Printf("[%s] 同步第 %d 组文件夹...", task.Name, i+1)
		if err := runRclone(task.Type, src, dst, pair, isManual); err != nil {
			handleError(task.Name, "Rclone 执行错误: "+err.Error(), isManual)
			return
		}

		if err := verify(src, dst, pair); err != nil {
			handleError(task.Name, err.Error(), isManual)
			return
		}
	}

	// 3. Post-Start
	for _, p := range task.ProcessManagement.PostStart {
		cmd := exec.Command("cmd", "/c", "start", "", ExpandPath(p))
		cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
		cmd.Start()
	}

	log.Printf("[%s] 任务顺利完成", task.Name)
	// 此处已移除成功提示弹窗
}

func handleError(taskName, msg string, isManual bool) {
	log.Printf("[%s] 错误: %s", taskName, msg)
	if isManual {
		ShowMsg("同步错误", "任务: "+taskName+"\n"+msg, true)
	}
}
