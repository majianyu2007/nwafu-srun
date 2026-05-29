# nwafu-srun bypass

独立 bypass 工具，复用 `pkg/srun` 与 `pkg/config`。

> 本工具**不包含**在 GitHub Release 预编译包中；二开或自用请本地 `go build`。

## 编译

```bash
go build -o bypass ./utils/bypass
```

## 用法

```bash
# 已在线，仅 bypass
./bypass -u your_username

# 从用户配置目录读取用户名（与主程序相同）
./bypass

# 完整流程
./bypass -u your_username -p your_password --login

# 对账号下所有在线设备生效绕过计费
./bypass -u your_username -a

# 指定配置文件
./bypass --config /path/to/config.json
```

> 默认仅会下线与本机 MAC 对应的会话（即仅使本机生效绕过计费，不影响其他设备）。
> 若希望该账号下的所有设备都同时生效绕过，则需要附加 `-a` 选项以让所有在线设备均触发踢线。
> 不需要高频循环执行——重连或断开后成功触发一次踢线即可。
> **提示**：设备重新连接校园网（如 Wi-Fi 断连、拔插网线）后，绕过可能会失效。建议在每次重连网络后重新运行工具，或登录自服务门户进行确认。

失败时会打印 `Error:` 与 `Hint:`（与主程序一致）。

## 退出码

- 0 成功
- 1 运行时错误
- 2 参数错误

## 环境变量

`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`（`--login` 时需要密码）。
