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

# 3) 保存配置；在交互菜单 6（设置）中开启 auto-auth / bypass 等，或编辑 JSON 的 "auto_auth"
./nwafu-srun -u USER -p PASS --save-config
```

## 编译

```bash
go build -o nwafu-srun .
go build -o utils/bypass/bypass ./utils/bypass
```

也可使用 `utils/build.sh` / `utils/build.bat`。

## 使用方式

### 非交互模式

满足以下任一条件时自动进入非交互流水线（`pre-logout? → login → bypass?`）：

- 命令行同时提供 `-u` 与 `-p`
- 环境变量 `NWAFU_SRUN_USERNAME` + `NWAFU_SRUN_PASSWORD`
- 配置文件中 `auto_auth: true` 且已保存用户名密码
- 已保存凭据且命令行指定了 `-f` 或 `-b`

```bash
./nwafu-srun -u USER -p PASS          # 登录
./nwafu-srun -u USER -p PASS -f       # 先登出再登录
./nwafu-srun -u USER -p PASS -b       # 登录后 bypass
./nwafu-srun -u USER -p PASS -f -b -a # 完整流水线并断开所有设备
```

### 交互模式

无凭据时进入菜单：

| 选项 | 功能 |
|------|------|
| 1 | 登录（已在线时询问是否覆盖） |
| 2 | 强制重登（先登出再登录） |
| 3 | 注销 |
| 4 | 状态查询 |
| 5 | Bypass 计费（询问是否踢光账户下所有会话） |
| 6 | 设置（保存凭据、切换 auto-auth / force / bypass / kick-all 并立即写入配置等） |
| 7 | 修改本会话凭据（用户名/密码） |
| 8 | 退出（`q` / `quit` / `exit` 亦可） |

输入用户名和密码后会询问是否立即登录；登录成功则出现保存凭据提示（`y` / `n` / `never`）。`Ctrl+D` 安静退出。

登录成功后可选择将凭据保存为配置文件。
如果你不希望每次都看到“是否保存凭据”的提示，可在提示中输入 `never` 永久关闭（设置菜单可恢复）。

### 自动认证

1. 交互模式登录成功后，选择保存配置并开启 `auto_auth`
2. 此后直接运行 `nwafu-srun` / `nwafu-srun.exe`（无参数）即可自动登录

在**设置菜单（6）**里也可切换 `force`（先登出）、`bypass`（登录后 bypass）、`kick-all`（等同 `-a`，会二次确认），与 `auto_auth` 一样会立即写入配置文件。

若配置里同时启用了 `force` 或 `bypass`，无参数启动时会先打印一行流水线提示并自动执行对应步骤；需要菜单时请传 `--no-config`。

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
  "all": false
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
| `-a` | 与 `-b` 配合：踢光账户下所有会话（bypass 真正生效所必需） |
| `--acid` | ac_id（默认 1） |
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

Bypass 的有效性依赖于一次性"重写"账户下**所有**在线会话的 user_mac 让 RADIUS accounting 状态错乱：

1. SSO 登入自服务门户，列出该账号当前所有在线会话（一般 ≤ 3）
2. 对每个目标会话各生成一个**随机假 MAC**，立即提交下线请求
3. 设备在 Portal 网关层并未真正注销，会立刻重新登记会话
4. 由于 accounting 状态被打乱，新登记出的会话通常**不计费**

注意：

- 只踢一部分会话（例如只踢自己 MAC 的那一条）通常不会触发不计费效果——bypass 真正生效需要**一次踢光**所有会话。
- 但"踢光所有会话"也意味着同账户下其他人的设备会被一起踢掉。出于这个副作用，**默认只踢自己 MAC 的会话**：
  - 命令行：要真正 bypass 必须显式加 `-a`
  - 交互菜单：选择 5 后会询问 *"Kick ALL sessions on this account?"*，回答 `y` 才全踢
- 也不需要"持续踢"：触发一次成功的全踢即可。

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

## crontab 示例

```cron
@reboot sleep 30 && NWAFU_SRUN_USERNAME=u NWAFU_SRUN_PASSWORD=p /path/to/nwafu-srun -f >> /tmp/nwafu-srun.log 2>&1
```

## 开发

```bash
go test ./...
go vet ./...
```

## 许可

MIT License
