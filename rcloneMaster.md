# RcloneMaster v1.0 技术手册

RcloneMaster 是一个专为 Windows 用户设计的增强型 Rclone 自动化管理工具。它解决了 Rclone 原生运行中的三大痛点：**环境变量不支持**、**进程占用导致同步失败**、以及**复杂的过滤规则配置**。

## 1. 核心特性
*   **配置重定向**：支持将任务清单放在同步盘（如 OneDrive/Dropbox），实现多机配置自动同步。
*   **环境变量解析**：全面支持 `%APPDATA%`、`%USERPROFILE%` 等系统变量。
*   **静默流水线**：同步前自动杀进程，同步后自动重启进程，全程无黑窗口干扰。
*   **双重校验**：不仅检查退出码，还执行带过滤规则的文件大小/数量比对。

---

## 2. 配置文件指南

程序由两个 YAML 文件驱动，实现了“环境”与“业务”的彻底解耦。

### 2.1 全局配置 `rcloneMaster.yaml`
此文件定义当前电脑的运行环境。

| 配置项 | 说明 |
| :--- | :--- |
| `master_config_path` | (可选) 若填写，程序将跳转到该路径加载最终的全局配置 |
| `rclone_path` | `rclone.exe` 的绝对路径，支持环境变量 |
| `task_config_path` | `task.yaml` 的路径，指向具体任务配置 |
| `log_dir` | 日志存储目录，按天生成 `.log` 文件 |
| `temp_dir` | 锁文件（.lock）存储目录，建议设为 AppData 或 Temp |
| `default_verify_level` | `1` 开启大小校验；`0` 关闭 |

配置示例：
```
global:
  master_config_path: "" 
  rclone_path: "D:/CloudSoft/CMDTools/Tools/rclone.exe"
  task_config_path: "D:/UsersData/Zakary/Documents/rcloneMaster/task.yaml"
  log_dir: "D:/UsersData/Zakary/Documents/rcloneMaster/logs"
  temp_dir: "D:/UsersData/Zakary/Documents/rcloneMaster/temp"
  default_verify_level: 1
```

### 2.2 任务配置 `task.yaml`
此文件定义具体的同步逻辑。

```yaml
tasks:
  - name: "everything"     # 任务唯一名称
    type: "Sync"                # Rclone 操作类型 (Sync/BiSync/Copy)
    folders:                    # 文件夹对列表
      - source: "C:/Source"
        dest: "D:/Dest"
        includes: []            # 白名单
        excludes: []            # 黑名单
    realtime: true              # 守护进程模式下是否实时监控
    schedule: "0 */2 * * *"     # Cron 表达式 (分 时 日 月 周)
    process_management:
      pre_kill: ["exe名"]       # 同步前强制关闭的进程列表
      post_start: ["路径"]      # 同步后启动的程序全路径
```

配置示例：
```
tasks:
  - name: "locallnk"
    type: "Sync"
    folders:
      - source: "C:/ProgramData/Microsoft/Windows/Start Menu/Programs/LocalLNK"
        dest: "D:/LocalLnk"
    process_management:
      pre_kill: []

  - name: "everything"
    type: "sync"
    folders:
      - source: "C:/Program Files/Everything"
        dest: "D:/Settings/EveryThing设置/1.4/Program"
        includes:
          - "/IbEverythingExt/**"
          - "/Everything.ini"
          - "/WindowsCodecs.dll"
      - source: "C:/Users/Zakary/AppData/Roaming/Everything"
        dest: "D:/Settings/EveryThing设置/1.4/AppData"
        includes: 
          - "/Everything.ico"
          - "/Everything.ini"
          - "/Filters.csv"
    process_management:
      pre_kill: ["Everything.exe"]
      post_start: ["C:/Program Files/Everything/Everything.exe"]
```

---

## 3. 过滤语法详解 (Includes & Excludes)

RcloneMaster 采用 **“白名单优先”** 策略：
1. 若 `includes` 不为空：只同步匹配到的文件。
2. 若 `includes` 为空且 `excludes` 不为空：同步除匹配到的文件外的所有内容。
3. 若均为空：全量同步。

### 3.1 关键符号说明
*   `*`：匹配单层目录下的文件/文件夹名（不跨越 `/`）。
*   `**`：递归匹配，代表任意深度的目录结构。
*   `/`：若出现在开头，表示从 source 的根目录开始匹配；否则匹配任何层级的同名项。

### 3.2 常用场景示例

| 语法示例 | 匹配结果 | 适用场景 |
| :--- | :--- | :--- |
| **`Everything.ini`** | 匹配任何位置的 Everything.ini | 简单的单配置文件同步 |
| **`/Everything.ini`** | 只匹配根目录下的配置文件 | 防止同名子目录文件干扰 |
| **`IbEverythingExt/**`** | 匹配该文件夹及其下所有子文件/子目录 | **同步插件、子模块的最推荐写法** |
| **`*.sav`** | 匹配所有 .sav 后缀的文件 | 提取游戏存档 |
| **`Cache/**`** | 排除所有层级中名为 Cache 的目录及其内容 | 减少无用垃圾同步 (Excludes) |
| **`Logs/*.log`** | 只同步 Logs 目录下的第一层日志 | 忽略旧的存档日志 |

---

## 4. 命令行操作 (CLI)

程序在编译时使用了 `-H windowsgui` 参数，所有的操作都不会产生黑色的 CMD 窗口，错误通过 Windows 对话框通报。

### 4.1 守护进程模式 (Daemon)
```bash
rcloneMaster.exe daemon
# 或者直接双击运行 exe
```
*   **功能**：在后台常驻，管理以下两类自动化行为：
  * 变动触发：对于 realtime: true 的任务，程序会持续监控文件夹。一旦检测到文件新增、修改或删除，将在 8 秒后自动触发备份。
  * 时间触发：对于配置了 schedule 的任务，程序将严格按照 Cron 表达式设定的时间点执行备份。

*   **注意**：实时监控仅对 source (源端) 有效。如果你修改了 dest (目标端) 的文件，实时监控不会触发（这是为了防止双向同步时的死循环）。

### 4.2 手动备份 (Backup)
```bash
rcloneMaster.exe backup [任务名]
```
*   **方向**：`Source` -> `Dest`。
*   **场景**：下班前手动触发一次重要工作数据的上传。

### 4.3 手动还原 (Restore)
```bash
rcloneMaster.exe restore [任务名]
```
*   **方向**：`Dest` -> `Source`。
*   **场景**：在新电脑上重装系统后，一键恢复软件配置。

---

## 5. 安全机制说明

1.  **文件锁 (`.lock`)**：
    每个任务在执行时会在 `temp_dir` 下生成一个 `.lock` 文件。如果前一个任务未完成，后一个任务（无论是定时还是手动）都会被自动阻断，防止文件占用导致数据损坏。
2.  **优雅终止进程**：
    `pre_kill` 逻辑：前 2 次尝试温柔关闭（让程序有时间存盘），后 3 次执行强制杀进程（`taskkill /F`）。
3.  **防抖处理 (Debounce)**：
    实时监控模式下，文件变动后会等待 8 秒。如果在 8 秒内有连续变动（如连续修改配置文件），程序只会在最后一次变动结束后触发一次同步，避免频繁启停 Rclone。
