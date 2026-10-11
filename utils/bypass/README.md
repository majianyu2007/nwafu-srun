# nwafu-srun bypass

只执行 bypass 的独立工具，复用 `pkg/srun` 与 `pkg/config`，配置文件与主程序共用。

本工具不包含在 GitHub Release 中，需自行编译：

```bash
go build -o bypass ./utils/bypass
```

## 用法

```bash
# 本机已在线，执行 bypass
./bypass -u USER -p PASS

# 用户名和密码从配置文件读取
./bypass

# 先注销、再登录、再执行 bypass
./bypass -u USER -p PASS --login

# 指定出口接口与源地址
./bypass -u USER -p PASS --login --bind-iface mv-student --bind-ip 10.130.6.94

# 作用于账号下所有设备
./bypass -u USER -p PASS -a

# 指定配置文件
./bypass --config /path/to/config.json
```

未使用 `--login` 时，程序先查询 Portal 在线状态，本机未在线则报错退出。

默认只处理本机 MAC 的会话，并在完成后从无感知认证中解绑本机 MAC。`-a` 处理账号下所有设备。配置文件中 `all` 为 `true` 而命令行未给出 `-a` 时，程序会要求确认；stdin 不是终端时直接以参数错误退出。

bypass 流程与注意事项见主程序 [README](../../README.md#bypass)。

## 参数

| 参数 | 说明 |
|------|------|
| `-u`, `--username` | 用户名 |
| `-p`, `--password` | 密码；用于自服务 SSO 与 `--login` |
| `--login` | 先注销再登录，然后执行 bypass |
| `-a`, `--all` | 作用于账号下所有设备 |
| `--retries N` | 下线后的校验重试次数，默认 3 |
| `--acid N` | Portal 的 `ac_id`，默认 1 |
| `--bind-ip IP` | 请求使用的本地源地址 |
| `--bind-iface IFACE` | 请求使用的出口接口（仅 Linux） |
| `--config PATH` | 指定配置文件 |
| `--no-config` | 不读取配置文件 |
| `-v`, `--verbose` | 在 stderr 输出调试信息 |
| `-h`, `--help` | 显示帮助 |

环境变量：`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`。

未提供密码时，自服务 SSO 改用仅用户名的方式，要求本机已通过 Portal 认证。

## 退出码

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | 运行时错误 |
| 2 | 参数错误 |

出错时在 stderr 输出 `Error:` 与 `Hint:`，与主程序一致。
