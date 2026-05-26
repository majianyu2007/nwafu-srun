# nwafu-srun bypass

独立 bypass 工具，复用 `pkg/srun` 与 `pkg/config`。

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

# 踢账户下所有设备
./bypass -u your_username -a

# 指定配置文件
./bypass --config /path/to/config.json
```

失败时会打印 `Error:` 与 `Hint:`（与主程序一致）。

## 退出码

- 0 成功
- 1 运行时错误
- 2 参数错误

## 环境变量

`NWAFU_SRUN_USERNAME`、`NWAFU_SRUN_PASSWORD`（`--login` 时需要密码）。
