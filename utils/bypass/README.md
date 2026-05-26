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

# 踢光账户下所有会话（bypass 真正生效需要这一步）
./bypass -u your_username -a

# 指定配置文件
./bypass --config /path/to/config.json
```

> 默认只踢与本机 MAC 匹配的会话以保护账户下其他人的设备。
> 但 RADIUS accounting 错乱手法只有"一次踢光所有会话"才会生效，所以要真正 bypass 必须加 `-a` / `--all`。
> 不需要循环执行——成功一次即可。

失败时会打印 `Error:` 与 `Hint:`（与主程序一致）。

## 退出码

- 0 成功
- 1 运行时错误
- 2 参数错误

## 环境变量

`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`（`--login` 时需要密码）。
