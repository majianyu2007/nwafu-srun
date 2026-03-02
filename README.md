# nwafu-srun

![CodeQL Badge](https://github.com/dingyx99/nwafu-srun/workflows/CodeQL/badge.svg)

原作者已经毕业了，因无法接触到后续的任何网络环境和参数变更，项目即日起停更，如遇使用上的问题，可 fork 后提 pr 进行修复，祝各位使用愉快～

西北农林科技大学深澜认证工具，现已使用 Go 语言重写，提供跨平台独立可执行文件。

该工具包含了原版 `main.py` 的交互式登录、查询信息和注销功能。此外，通过 `--force` 参数，可以实现原版 `login.py` 的效果（适用于需要自动认证的脚本环境）。

## 编译与使用方法

请先运行对应平台的编译脚本，或是自己使用 `go build` 编译（Windows 下可以运行 `build.bat`，Linux/macOS 下运行 `build.sh`）。

编译成功后，将会生成 `nwafu-srun.exe`（或其它无后缀文件）。请按照如下指令运行程序：## Usage

```bash
# Interactive mode
./nwafu-srun -u your_username -p your_password

# Force login/logout mode (no interactive prompt, equivalent to login.py)
./nwafu-srun -u your_username -p your_password -f

# Troubleshooting verbose mode (dumps out HTTP requests and responses)
./nwafu-srun -u your_username -p your_password -f -v
```

### Options
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
