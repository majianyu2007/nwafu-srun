# nwafu-srun

![CodeQL](https://github.com/majianyu2007/nwafu-srun/workflows/CodeQL/badge.svg)

西北农林科技大学深澜认证工具，使用 Go 语言编写，提供跨平台独立可执行文件。

该工具提供了交互式登录、查询信息和注销功能。此外，通过 `--force` 参数，可以直接执行认证（适用于需要自动认证的脚本环境）。

## 编译与使用方法

请先运行对应平台的编译脚本（位于 `utils/` 目录下），或直接使用 `go build -o nwafu-srun main.go` 编译。

编译成功后，将会生成 `nwafu-srun`（Windows 下为 `nwafu-srun.exe`）。

## Usage

```bash
# Interactive mode (credentials optional, will prompt if not provided)
./nwafu-srun
./nwafu-srun -u your_username -p your_password

# Force login/logout mode (no interactive prompt, for script/cron use)
./nwafu-srun -u your_username -p your_password -f

# Verbose mode (prints HTTP requests and responses)
./nwafu-srun -u your_username -p your_password -f -v

# Show help
./nwafu-srun -h
```

### Options
**特色功能**：当因为断网导致无法解析 `portal.nwafu.edu.cn` 时，程序会自动尝试连接其备用 IP 地址 `172.26.8.11`。

测试环境：
- Go 1.20+ 
- Windows 10/11, macOS, Linux


## 已知问题

* 注销功能因为深澜系统的问题，不能正常使用；
* 刚认证之后无法正常获取用户信息；

### 自动认证配置 (Linux / macOS crontab)

如果您希望在路由器 (如 OpenWrt)、NAS 或 Linux 服务器上实现断网自动重连和定时重连，您可以使用 `crontab` 来定时执行此程序，并在执行时带上 `--force` 或 `-f` 标签。

1. 打开终端，输入 `crontab -e` 以编辑当前用户的定时任务。
2. 在文件末尾添加以下两行（请将 `/path/to/nwafu-srun` 替换为您实际存放该程序的绝对路径）：

```cron
# 开机时运行一次自动认证
@reboot sleep 30 && /path/to/nwafu-srun -u your_username -p your_password -f >> /tmp/nwafu-srun.log 2>&1

# 每天早上 6:00 定时运行自动认证
0 6 * * * /path/to/nwafu-srun -u your_username -p your_password -f >> /tmp/nwafu-srun.log 2>&1
```

> **注意**：开机启动时（`@reboot`）建议加上 `sleep 30` 延时，确保网络接口和路由表已经初始化完毕后再执行认证程序。日志会输出到 `/tmp/nwafu-srun.log` 中以便日后排查问题。

---

*本项目基于 [dingyx99/nwafu-srun](https://github.com/dingyx99/nwafu-srun) 的算法重构，感谢原作者的研究。*

## 致谢

[vincentimba/shenlan_xauat](https://github.com/vincentimba/shenlan_xauat): 项目灵感（其实是不想实现那个加密算法了）

## 许可

MIT License
