# nwafu-srun

![CodeQL](https://github.com/majianyu2007/nwafu-srun/workflows/CodeQL/badge.svg)

西北农林科技大学深澜认证工具，使用 Go 语言编写，提供跨平台独立可执行文件。

支持交互式登录、信息查询、注销、bypass 计费，以及**配置文件自动认证**（无参数运行即可登录）。

## 快速开始

```bash
# 1) 编译
go build -o nwafu-srun .

# 2) 直接登录
./nwafu-srun -u USER -p PASS

# 3) 保存配置；在交互菜单 7（设置）中开启 auto-auth / bypass 等，或编辑 JSON 的 "auto_auth"
./nwafu-srun -u USER -p PASS --save-config
```

## 编译

```bash
go build -o nwafu-srun .
go build -o utils/bypass/bypass ./utils/bypass
```

也可使用 `utils/build.sh` / `utils/build.bat`。

## 使用方式

### 命令行直连（非交互模式）

若满足以下任一条件，程序将直接执行登录/注销/绕过流程，不会进入交互式菜单：

- 命令行指定了 `-u` 和 `-p` 参数
- 环境变量中设置了 `NWAFU_SRUN_USERNAME` 和 `NWAFU_SRUN_PASSWORD`
- 配置文件中 `auto_auth` 为 `true` 且已保存用户名和密码
- 已保存账号凭据，且命令行指定了 `-f`（重新登录）或 `-b`（绕过计费）参数

```bash
./nwafu-srun -u USER -p PASS          # 直接登录
./nwafu-srun -u USER -p PASS -f       # 强制重新登录（先登出已有会话再登录）
./nwafu-srun -u USER -p PASS -b       # 登录并执行计费绕过（默认仅下线本机 MAC 会话）
./nwafu-srun -u USER -p PASS -f -b -a # 强制重新登录，并绕过该账号下所有在线设备的计费
```

### 交互模式

无凭据时进入菜单：

| 选项 | 功能 |
|------|------|
| 1 | 登录（已在线时询问是否覆盖） |
| 2 | 强制重登（先登出再登录） |
| 3 | 注销（根据配置选择常规 Portal 注销或自服务直接踢下线本机 MAC） |
| 4 | 状态查询 |
| 5 | Bypass 计费（询问是否踢光账户下所有会话） |
| 6 | 在线会话管理（连接自服务列出所有在线会话，可手动选择踢下线指定设备或一键全踢） |
| 7 | 设置（保存凭据、切换 auto-auth / force / bypass / kick-all / logout-mode 并立即写入配置等） |
| 8 | 修改本会话凭据（用户名/密码） |
| 0 | 退出（`q` / `quit` / `exit` 亦可） |

输入用户名和密码后会询问是否立即登录；登录成功则出现保存凭据提示（`y` / `n` / `never`）。`Ctrl+D` 安静退出。

登录成功后可选择将凭据保存为配置文件。
如果你不希望每次都看到“是否保存凭据”的提示，可在提示中输入 `never` 永久关闭（设置菜单可恢复）。

### 自动认证

1. 交互模式登录成功后，选择保存配置并开启 `auto_auth`
2. 此后直接运行 `nwafu-srun` / `nwafu-srun.exe`（无参数）即可自动登录

在 **设置菜单（7）** 里也可切换 `force`（先登出）、`bypass`（登录后 bypass）、`kick-all`（等同 `-a`，会二次确认）、`logout-mode`（注销模式：portal 或 selfservice，后者用于强踢本机 MAC 会话以规避无感知自动重新登录），与 `auto_auth` 一样会立即写入配置文件。

若配置文件中启用了 `auto_auth` 且设置了 `force` 或 `bypass`，无参数启动时将自动执行对应步骤。若需要进入菜单，请传入 `--no-config` 或 `-m` 参数。

或通过命令行一次性写入配置：

```bash
./nwafu-srun -u USER -p PASS -f --save-config
# 编辑生成的 JSON，将 "auto_auth" 设为 true
```

## 配置文件

配置文件**始终**保存在当前用户目录（与可执行文件位置无关）：

| 平台 | 路径 |
|------|------|
| Linux/macOS | `~/.config/nwafu-srun/config.json`（或 `$XDG_CONFIG_HOME/nwafu-srun/config.json`） |
| Windows | `%AppData%\nwafu-srun\config.json` |

读取：`--config <path>` 指定其他文件；否则使用上述用户目录。`--no-config` 禁用。

示例：

```json
{
  "version": 1,
  "username": "your_id",
  "password": "your_password",
  "acid": "1",
  "auto_auth": true,
  "force": false,
  "bypass": false,
  "all": false,
  "logout_mode": "portal"
}
```

**安全说明**：密码以**明文**存储，文件权限为 `0600`（Windows 另设隐藏属性）。请勿在共享账户或公共计算机上保存配置。可使用 `--no-config` 禁用读写。

### 配置相关 CLI

| Flag | 说明 |
|------|------|
| `--config <path>` | 指定配置文件 |
| `--no-config` | 忽略所有配置文件 |
| `--save-config` | 将当前合并后的配置写入用户目录（可用已加载的 config 提供凭据；`all` 仅在 `bypass` 为真时写入） |

## 选项一览

| Flag | 说明 |
|------|------|
| `-u`, `-p` | 用户名 / 密码 |
| `-f` | 登录前登出 |
| `-b` | 登录后 bypass（默认仅踢自己 MAC 的会话） |
| `-a` | 与 `-b` 配合：对账号下所有在线设备生效绕过计费（默认仅对本机生效） |
| `--acid` | ac_id（默认 1） |
| `--logout-mode` | 注销模式：`portal`（默认，网页注销）或 `selfservice`（自服务踢下线本机 MAC 会话） |
| `-v` | 详细日志（stderr） |
| `-h` | 帮助 |

环境变量：`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`。

## Bypass 命令行工具

源码位于 `utils/bypass/`，**不会**随 GitHub Release 发布预编译包；需要请自行编译：

```bash
go build -o utils/bypass/bypass ./utils/bypass
```

```bash
./utils/bypass/bypass -u USER           # 已在线时仅 bypass
./utils/bypass/bypass --login -u USER -p PASS
./utils/bypass/bypass                   # 从配置文件读取 username
```

详见 [utils/bypass/README.md](utils/bypass/README.md)。

### Bypass 工作原理

Bypass 的有效性依赖于断开在线会话并触发 RADIUS 计费服务状态不同步（Accounting Desync）：

1. 通过 SSO 单点登录方式接入自服务门户，列出该账号当前的所有在线会话（通常不超过 3 个）。
2. 对选定的在线会话，使用一个**固定的无效假 MAC** `02:00:00:00:00:00` 提交下线请求。
3. 设备在本地 Portal 网关层并未真正断开，但 RADIUS 计费服务器已将其视为下线并停止统计流量。
4. 随后重新登记或维持在线的会话，通常便不再计入费用。

注意：

- **单设备与全账号绕过**：默认仅会踢除与本机 MAC 对应的会话（即仅使本机生效绕过计费，不影响其他设备）。若希望该账号下的**所有设备**都同时生效绕过，则需要附加 `-a` 参数（或在菜单中确认全踢），使该账号下的所有在线设备均触发踢线。
- **设备短暂断开**：被踢线的设备会经历非常短暂的下线并自动重新登记上线。该机制并不需要“持续高频踢线”，只需在登录或断网重连后执行一次成功的踢线操作即可。
- **重连失效风险**：当设备断网并重新连接校园网（例如 Wi-Fi 断开重连、有线网线拔插）后，系统会建立全新的计费会话，绕过可能会因此失效。建议在每次网络重连后重新执行一次本工具，或者手动登录自服务门户确认当前的计费状态。

> **免责声明**：Bypass 功能的实际效果取决于校园网认证系统的策略与配置，本项目不保证其在所有环境、所有时间均能生效。使用者应在使用后自行验证计费是否已被绕过，因 bypass 失败产生的流量费用与本项目无关。

## 错误自检

失败时除 `Error:` 外会打印 `Hint:` 建议：

| 错误类型 | 含义 | 建议 |
|----------|------|------|
| `not online` | 未认证 | 先执行登录（菜单 1） |
| `portal unreachable` | 无法连接认证页 | 检查校园网 / DNS |
| `SSO redirected to login` | 自服务 SSO 失败 | 先 Portal 登录再 bypass |
| `local MAC undetected` | 无法获取本机 MAC 地址 | 先登录获取 MAC，或使用 `-a` 全踢 |
| `no session matched the given MAC` | 没有匹配本机 MAC 的会话 | 确认本机已在线，或使用 `-a` 全踢 |
| `no online sessions to kick` | 自服务里看不到任何在线会话 | 先 Portal 登录使会话注册到 RADIUS |
| `auth failed` | 账号密码错误 | 检查凭据与 `--acid` |
| `kick session failed` / `context deadline exceeded` / `198.18.x.x` | 代理软件（TUN 模式 / fake-IP）拦截了校内流量 | 在 Clash / Mihomo / v2rayN / Surge 等代理软件里把 `service.nwafu.edu.cn`、`portal.nwafu.edu.cn` 与 `172.26.0.0/16` 加入直连 / bypass 规则，或临时关闭代理后重试 |

使用 `-v` 可在 stderr 查看 HTTP 详情。

## 退出码

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | 运行时错误（认证/网络/bypass 失败） |
| 2 | 参数或配置错误 |

## 后台运行与断线重连守护

本程序未内置后台守护进程（Daemon）模式，但可以通过系统级的定时任务或服务管理器轻松实现**开机自启**和**断线重连检测**。

### 1. 使用 Cron 定时检查（Linux / macOS 推荐）

配置 Cron 定时任务，每分钟检查一次网络状态，如果掉线则自动重连。

编辑 crontab (`crontab -e`)，添加以下规则：

```cron
# 每分钟执行一次检测与自动认证（请使用绝对路径，且必须已在设置菜单中保存凭据并开启 auto_auth）
* * * * * /path/to/nwafu-srun >> /tmp/nwafu-srun-keep.log 2>&1
```

### 2. 使用 Systemd 作为服务常驻后台（Linux）

在 `/etc/systemd/system/nwafu-srun.service` 创建服务文件：

```ini
[Unit]
Description=NWAFU SRUN Portal Autoconnect Client
After=network.target

[Service]
Type=simple
ExecStart=/path/to/nwafu-srun
# 如果断开，等待 10 秒后自动重启（配合已保存的 auto_auth=true 规则）
Restart=always
RestartSec=10
User=your_username
Environment=HOME=/home/your_username

[Install]
WantedBy=multi-user.target
```

然后启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable nwafu-srun.service
sudo systemctl start nwafu-srun.service
```

### 3. 使用 Windows 任务计划程序（Windows）

1. 打开**任务计划程序** (Task Scheduler)。
2. 创建一个新任务，触发器设置为 **“当任何用户登录时”**。
3. 操作设置为 **“启动程序”**，指向 `nwafu-srun.exe`，不加任何参数（前提是已使用交互菜单 `7` 保存配置并开启了 `auto_auth`）。
4. 在**设置**选项卡中，勾选 **“如果任务失败，重新启动”**，并可以配置为每小时或断网时重复运行。

或者使用第三方小工具 `NSSM` (nssm.cc) 将其注册为 Windows 后台 Service 服务，配置为 `Restart: always`。

## 开发

```bash
go test ./...
go vet ./...
```

## 许可

MIT License
