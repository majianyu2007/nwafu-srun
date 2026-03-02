# nwafu-srun

![CodeQL Badge](https://github.com/dingyx99/nwafu-srun/workflows/CodeQL/badge.svg)

作者已经毕业了，因无法接触到后续的任何网络环境和参数变更，项目即日起停更，如遇使用上的问题，可 fork 后提 pr 进行修复，祝各位使用愉快～

西北农林科技大学深澜认证工具，已使用 Go 语言重写，无任何系统依赖（不再需要 Python），提供跨平台独立可执行文件。

该工具包含了原版 `main.py` 的交互式登录、查询信息和注销功能。此外，通过 `--force` 参数，可以实现原版 `login.py` 的效果（适用于需要自动认证的脚本环境）。

## 编译与使用方法

请先运行对应平台的编译脚本，或是自己使用 `go build` 编译（Windows 下可以运行 `build.bat`，Linux/macOS 下运行 `build.sh`）。

编译成功后，将会生成 `nwafu-srun.exe`（或其它无后缀文件）。请按照如下指令运行程序：

`./nwafu-srun -u <你的认证用户名> -p <你的密码>`

或

`./nwafu-srun --username=<你的认证用户名> --password=<你的密码>`

若你是写在自动任务（如 `crontab` 或开机自启动脚本）中并且不希望弹出交互式菜单，可以添加 `--force` 参数（相当于原 `login.py` 的行为）：

`./nwafu-srun -u user -p pass --force`

**特色功能**：当因为断网导致无法解析 `portal.nwafu.edu.cn` 时，程序会自动尝试连接其备用 IP 地址 `172.26.8.11`。

测试环境：
- Go 1.20+ 
- Windows 10/11, macOS, Linux


## 已知问题

* 注销功能因为深澜系统的问题，不能正常使用；
* 刚认证之后无法正常获取用户信息；

## 致谢

[vincentimba/shenlan_xauat](https://github.com/vincentimba/shenlan_xauat): 项目灵感（其实是不想实现那个加密算法了）

## 许可

MIT License
