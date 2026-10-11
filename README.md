# nwafu-srun

![CodeQL](https://github.com/majianyu2007/nwafu-srun/workflows/CodeQL/badge.svg)

西北农林科技大学深澜（Srun）校园网认证命令行工具，Go 编写，各平台均为单个静态可执行文件。

功能：

- Portal 登录、注销、在线状态查询
- 基于配置文件的自动认证（无参数运行即登录）
- 自服务门户：在线会话查询与下线、无感知认证（MAC-Auth）绑定与解绑
- 计费 bypass
- 绑定出口接口或源地址（适用于路由器、macvlan 多账号场景）

## 获取

从 [Releases](https://github.com/majianyu2007/nwafu-srun/releases) 下载对应平台的可执行文件，或自行编译（需要 Go，版本见 `go.mod`）：

```bash
go build -o nwafu-srun .
```

`utils/build.sh`、`utils/build.bat` 会同时编译主程序与 `utils/bypass`，输出文件名带平台后缀。

## 快速开始

```bash
# 登录
./nwafu-srun -u USER -p PASS

# 保存凭据到配置文件
./nwafu-srun -u USER -p PASS --save-config

# 进入交互菜单，在“7 设置”中开启 auto-auth
./nwafu-srun -m

# 此后无参数运行即自动登录
./nwafu-srun
```

## 运行模式

### 非交互模式

满足以下任一条件时，程序执行登录流程后直接退出，不等待任何输入，可用于脚本、cron、systemd：

- 命令行同时给出 `-u` 与 `-p`
- 配置文件中 `auto_auth` 为 `true` 且已保存用户名和密码（未指定 `-m`）
- 已有凭据（来自配置文件或环境变量），且命令行给出 `-f` 或 `-b`

执行顺序：`-f` 时先注销 → 登录 → `-b` 时执行 bypass。

```bash
./nwafu-srun -u USER -p PASS                  # 登录
./nwafu-srun -u USER -p PASS -f               # 先注销再登录
./nwafu-srun -u USER -p PASS -b               # 登录后对本机执行 bypass
./nwafu-srun -u USER -p PASS -b --retries 5   # bypass 校验最多重试 5 次
./nwafu-srun -u USER -p PASS -f -b -a         # 先注销再登录，对账号下所有设备执行 bypass
```

### 自服务操作

以下参数各自完成一次操作后退出，不执行登录。需要用户名和密码（来自命令行、环境变量或配置文件），自服务 SSO 使用密码认证，因此本机未在线时同样可用。

```bash
./nwafu-srun --sessions        # 以 JSON 输出账号下的在线会话
./nwafu-srun --mac-auth-list   # 以 JSON 输出已绑定无感知认证的 MAC
./nwafu-srun --bind-mac        # 将本机 MAC 绑定到无感知认证
./nwafu-srun --unbind-mac      # 将本机 MAC 从无感知认证解绑
```

输出示例：

```json
[
  {
    "id": "123456",
    "mac": "aa:bb:cc:dd:ee:ff"
  }
]
```

本机 MAC 的获取顺序：Portal 在线信息 → `--bind-iface` 指定接口的硬件地址。两者都取不到时返回 `local MAC undetected`。绑定和解绑提交后会重新读取列表确认结果，未生效时返回 `MAC-Auth update failed`。

### 交互模式

未保存凭据、或指定 `-m` / `--menu` 时进入交互菜单：

| 选项 | 功能 |
|------|------|
| 1 | 登录；已在线时询问是否重新登录 |
| 2 | 强制重新登录（先注销再登录） |
| 3 | 注销，方式由 `logout_mode` 决定 |
| 4 | 查询在线状态 |
| 5 | Bypass；执行前询问是否作用于账号下所有设备 |
| 6 | 管理在线会话：列出自服务中的会话，可逐个或全部下线 |
| 7 | 设置：auto-auth、force、bypass、kick-all、logout-mode、保存提示，修改后立即写入配置文件 |
| 8 | 修改本次运行使用的用户名与密码 |
| 0 | 退出（也可输入 `q` / `quit` / `exit`） |

登录成功后会询问是否保存凭据，可选 `y` / `n` / `never`。选择 `never` 后不再提示，可在设置菜单中恢复。`Ctrl+D` 退出程序。

## 配置文件

默认路径位于当前用户的配置目录，与可执行文件所在位置无关：

| 平台 | 路径 |
|------|------|
| Linux | `~/.config/nwafu-srun/config.json`（设置了 `$XDG_CONFIG_HOME` 时位于其下） |
| macOS | `~/Library/Application Support/nwafu-srun/config.json` |
| Windows | `%AppData%\nwafu-srun\config.json` |

`--config <path>` 指定其他文件，`--no-config` 不读写任何配置文件。

参数优先级：命令行 > 环境变量 > 配置文件。

```json
{
  "version": 1,
  "username": "your_id",
  "password": "your_password",
  "acid": "1",
  "bind_ip": "10.130.6.94",
  "bind_interface": "mv-student",
  "auto_auth": true,
  "force": false,
  "bypass": false,
  "all": false,
  "logout_mode": "portal",
  "retries": 3
}
```

| 字段 | 说明 |
|------|------|
| `auto_auth` | 无参数运行时直接执行非交互登录 |
| `force` | 登录前先注销，等同 `-f` |
| `bypass` | 登录后执行 bypass，等同 `-b` |
| `all` | bypass 作用于账号下所有设备，等同 `-a`；仅在 `bypass` 为 `true` 时写入 |
| `logout_mode` | `portal` 或 `selfservice`，见下文 |
| `retries` | bypass 校验重试次数，等同 `--retries` |
| `save_prompt_disabled` | 登录后不再询问是否保存凭据 |

`--save-config` 将当前合并后的设置写入配置文件后退出。

**安全说明**：密码以明文保存，文件权限为 `0600`，Windows 下另设隐藏属性。不要在共用的计算机或账户上保存配置。

### 注销模式

- `portal`（默认）：调用 Portal 注销接口。
- `selfservice`：登录自服务门户，下线与本机 MAC 匹配的所有会话。适用于已绑定无感知认证、Portal 注销后会被自动重新认证的设备。

## 参数

| 参数 | 说明 |
|------|------|
| `-u`, `--username` | 用户名 |
| `-p`, `--password` | 密码 |
| `-f`, `--force` | 登录前先注销 |
| `-b`, `--bypass` | 登录后执行 bypass，默认只作用于本机 |
| `-a`, `--all` | 与 `-b` 配合，作用于账号下所有设备 |
| `--retries N` | bypass 下线后的校验重试次数，默认 3 |
| `-m`, `--menu` | 强制进入交互菜单 |
| `--acid N` | Portal 的 `ac_id`，默认 1 |
| `--logout-mode` | 注销模式：`portal` 或 `selfservice` |
| `--bind-ip IP` | Portal 与自服务请求使用的本地源地址 |
| `--bind-iface IFACE` | Portal 与自服务请求使用的出口接口（仅 Linux） |
| `--sessions` | 输出在线会话（JSON）后退出 |
| `--mac-auth-list` | 输出无感知认证 MAC 列表（JSON）后退出 |
| `--bind-mac` | 绑定本机 MAC 到无感知认证后退出 |
| `--unbind-mac` | 从无感知认证解绑本机 MAC 后退出 |
| `--config PATH` | 指定配置文件 |
| `--no-config` | 不读写配置文件 |
| `--save-config` | 保存配置后退出 |
| `-v`, `--verbose` | 在 stderr 输出 HTTP 请求等调试信息 |
| `-h`, `--help` | 显示帮助 |

环境变量：`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`。

## Bypass

### 流程

1. 使用账号密码通过 SSO 登录自服务门户，读取在线会话列表。
2. 按 MAC 对会话分组。同一 MAC 只有一个会话时视为已处于 bypass 状态，不做处理；有多个会话时选取按 ID 排序的中间一个作为目标。
3. 以随机生成的本地管理单播 MAC 提交下线请求。
4. 重新读取会话列表，确认目标会话已消失；未消失则刷新 CSRF 令牌后重试，最多 `--retries` 次。
5. 未使用 `-a` 时，从无感知认证列表中解绑本机 MAC，防止网关为该 MAC 重新建立计费会话。解绑失败只输出警告，不影响 bypass 结果。
6. 输出操作后的在线会话列表。

默认只处理本机 MAC 的会话。`-a` 处理账号下所有 MAC，会影响共用该账号的其他设备；在设置菜单中开启 kick-all 时会要求确认。

### 注意事项

- 只需在每次登录后执行一次，不需要循环执行。
- 设备重新接入网络（Wi-Fi 重连、网线插拔）后会建立新的会话，需要重新执行。
- 若此前依赖无感知认证自动上线，bypass 解绑后需重新使用 Portal 登录，或执行 `--bind-mac` 重新绑定。

> **免责声明**：Bypass 的效果取决于校园网认证系统的策略，本项目不保证在任何环境下都有效。请自行到自服务门户核实计费状态，因 bypass 未生效产生的费用与本项目无关。

### 独立 bypass 工具

`utils/bypass` 是只执行 bypass 的精简版本，不包含在 Release 中，需自行编译：

```bash
go build -o utils/bypass/bypass ./utils/bypass
```

用法见 [utils/bypass/README.md](utils/bypass/README.md)。

## 路由器与 macvlan

路由器上有多个上游接口时，可用 `--bind-iface` / `--bind-ip` 将认证相关请求固定到指定接口，避免 Portal、SSO、自服务请求从错误的出口发出。

典型场景：在校园网接口 `eth0` 上创建 macvlan 子接口 `mv-student`，用学号账号在该子接口上认证以获得 IPv6，原有 IPv4 路由与分流保持不变。

```bash
./nwafu-srun -u STUDENT_ID -p PASSWORD --bind-iface mv-student --bind-ip 10.130.6.94
```

- `--bind-iface` 通过 `SO_BINDTODEVICE` 绑定出口接口，仅 Linux 支持；Linux 5.7 之前的内核需要 root 或 `CAP_NET_RAW` 权限。
- `--bind-ip` 指定连接的本地源地址，各平台均可使用。
- 两者作用于 Portal 登录、状态查询、注销、SSO、bypass 及自服务操作。存在策略路由时建议同时指定。
- 指定 `--bind-iface` 且 Portal 未返回 MAC 时，程序使用该接口的硬件地址作为本机 MAC。

## 错误提示

出错时程序在 stderr 输出 `Error:`，已知错误另附 `Hint:`。加 `-v` 可查看 HTTP 详情。

| 错误 | 含义 | 处理 |
|------|------|------|
| `not online` | 本机未认证 | 先登录 |
| `portal unreachable` | 无法连接 Portal | 检查网络连接与 DNS |
| `self-service unreachable` | 无法连接自服务门户 | 检查代理软件的直连规则 |
| `SSO redirected to login` | 自服务 SSO 被拒绝 | 检查密码，或先在 Portal 登录 |
| `CSRF parse failed` | 自服务页面解析失败 | 页面结构可能已变化，加 `-v` 查看响应 |
| `local MAC undetected` | 无法确定本机 MAC | 先登录，或指定 `--bind-iface`，或使用 `-a` |
| `no session matched the given MAC` | 没有与本机 MAC 匹配的会话 | 确认本机在线，或使用 `-a` |
| `no online sessions to kick` | 自服务中没有在线会话 | 确认已登录 |
| `MAC-Auth update failed` | 无感知认证绑定或解绑未生效 | 用 `--mac-auth-list` 检查，或在自服务网页上操作 |
| `auth failed` | 认证失败 | 检查用户名、密码和 `--acid` |
| `kick session failed` | 下线请求失败 | 重试，加 `-v` 查看详情 |

错误信息中含有 `198.18.`、`connection reset`、`forcibly closed` 或请求超时，通常是 TUN 模式或 fake-IP 模式的代理软件（Clash、Mihomo、v2rayN、Surge 等）接管了校内流量。请将 `portal.nwafu.edu.cn`、`service.nwafu.edu.cn` 和 `172.26.0.0/16` 加入直连规则，或临时关闭代理。

## 退出码

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | 运行时错误（认证、网络、bypass、自服务操作失败） |
| 2 | 参数或配置错误 |

## 定时运行

程序不常驻后台，每次运行完成一次认证即退出。需要掉线自动重连时，可由系统定时任务周期性调用。以下示例均要求已保存凭据并开启 `auto_auth`。

### cron（Linux / macOS）

```cron
*/5 * * * * /path/to/nwafu-srun >> /tmp/nwafu-srun.log 2>&1
```

### systemd timer（Linux）

`/etc/systemd/system/nwafu-srun.service`：

```ini
[Unit]
Description=NWAFU Srun portal authentication
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=your_username
ExecStart=/path/to/nwafu-srun
```

`/etc/systemd/system/nwafu-srun.timer`：

```ini
[Unit]
Description=Run nwafu-srun periodically

[Timer]
OnBootSec=30s
OnUnitActiveSec=5min

[Install]
WantedBy=timers.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now nwafu-srun.timer
```

配置文件按 `User=` 指定用户的配置目录读取。也可在 `ExecStart` 中加 `--config /path/to/config.json` 指定。

### Windows 任务计划程序

新建任务，触发器选择“登录时”或按固定间隔重复，操作为启动 `nwafu-srun.exe`，不加参数。

## 开发

```bash
go vet ./...
go test ./...
```

推送 `v*` 标签后，GitHub Actions 运行测试、编译各平台静态二进制并创建 Release。

## 许可

MIT License
