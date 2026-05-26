# nwafu-srun

![CodeQL](https://github.com/majianyu2007/nwafu-srun/workflows/CodeQL/badge.svg)

西北农林科技大学深澜认证工具，使用 Go 语言编写，提供跨平台独立可执行文件。

该工具提供了交互式登录、查询信息、注销、bypass 计费，以及**配置文件自动认证**（双击 exe 即可登录）。

## 快速开始

```bash
# 1) 编译
go build -o nwafu-srun .

# 2) 直接登录
./nwafu-srun -u USER -p PASS

# 3) 保存配置并启用自动认证（交互菜单中也可设置）
./nwafu-srun -u USER -p PASS --save-config
```

## 编译

```bash
go build -o nwafu-srun .
go build -o utils/bypass/bypass ./utils/bypass
```

或使用 `utils/build.sh` / `utils/build.bat`。

## 使用方式

### 非交互模式

在以下任一情况下自动进入非交互流水线（`pre-logout? → login → bypass?`）：

- 命令行同时提供 `-u` 与 `-p`
- 环境变量 `NWAFU_SRUN_USERNAME` + `NWAFU_SRUN_PASSWORD`
- 配置文件中 `auto_auth: true` 且已保存用户名密码
- 已保存凭据且命令行指定了 `-f` 或 `-b`

```bash
./nwafu-srun -u USER -p PASS          # 登录
./nwafu-srun -u USER -p PASS -f       # 先登出再登录
./nwafu-srun -u USER -p PASS -b       # 登录后 bypass
./nwafu-srun -u USER -p PASS -f -b -a # 全流水线 + 踢全部设备
```

### 交互模式

无凭据时进入菜单：

| 选项 | 功能 |
|------|------|
| 1 | 登录（已在线时会询问是否覆盖） |
| 2 | 强制重登（先登出再登录） |
| 3 | 注销 |
| 4 | 状态查询 |
| 5 | Bypass 计费（会询问是否踢全部设备） |
| 6 | 设置（保存配置、auto-auth、查看/删除配置等） |
| 7 | 退出 |

登录成功后可选择将凭据保存为配置文件。
如果你不希望每次都看到“是否保存凭据”的提示，可在提示中输入 `never` 永久关闭（设置菜单可恢复）。

### 双击自动认证

1. 交互模式登录成功后，选择保存配置并开启 `auto_auth`
2. 下次在同一目录双击 `nwafu-srun` / `nwafu-srun.exe`（无参数）即可自动登录

或使用命令行一次性写入配置：

```bash
./nwafu-srun -u USER -p PASS -f --save-config
# 编辑生成的 JSON，将 "auto_auth" 设为 true
```

## 配置文件

配置文件**始终**保存在当前用户目录（与 exe 安装位置无关）：

| 平台 | 路径 |
|------|------|
| Linux/macOS | `~/.config/nwafu-srun/config.json`（或 `$XDG_CONFIG_HOME/nwafu-srun/config.json`） |
| Windows | `%AppData%\nwafu-srun\config.json` |

读取：`--config <path>` 指定其它文件；否则使用上述用户目录。`--no-config` 禁用。

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

**安全说明**：密码以**明文**存储，文件权限为 `0600`（Windows 另设隐藏属性）。请勿在共享账户或公共电脑上保存配置。可用 `--no-config` 禁用读写。

### 配置相关 CLI

| Flag | 说明 |
|------|------|
| `--config <path>` | 指定配置文件 |
| `--no-config` | 忽略所有配置文件 |
| `--save-config` | 将当前 `-u/-p/-f/-b/-a` 写入用户配置目录后退出 |

## 选项一览

| Flag | 说明 |
|------|------|
| `-u`, `-p` | 用户名 / 密码 |
| `-f` | 登录前登出 |
| `-b` | 登录后 bypass |
| `-a` | bypass 时踢账户下所有设备 |
| `--acid` | ac_id（默认 1） |
| `-v` | 详细日志（stderr） |
| `-h` | 帮助 |

环境变量：`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`。

## Bypass 小工具

```bash
./utils/bypass/bypass -u USER           # 已在线时仅 bypass
./utils/bypass/bypass --login -u USER -p PASS
./utils/bypass/bypass                   # 从配置文件读取 username
```

详见 [utils/bypass/README.md](utils/bypass/README.md)。

## 错误自检

失败时除 `Error:` 外会打印 `Hint:` 建议：

| 错误类型 | 含义 | 建议 |
|----------|------|------|
| `not online` | 未认证 | 先执行登录（菜单 1） |
| `portal unreachable` | 无法连接认证页 | 检查校园网 / DNS |
| `SSO redirected to login` | 自服务 SSO 失败 | 先 Portal 登录再 bypass |
| `local MAC undetected` | 读不到本机 MAC | 先登录，或用 `-a` |
| `no session matched` | 没有匹配 MAC 的会话 | 确认在线或用 `-a` |
| `auth failed` | 账号密码错误 | 检查凭据与 `--acid` |

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
