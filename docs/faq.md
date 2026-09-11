# 常见问题 / 故障排查（FAQ）

## 服务端

### 怎么更新到最新版本？

```bash
sudo skyfire update          # 等同 skyfired update
skyfire update -check        # 只检查
sudo skyfire update -version v0.6.0   # 固定版本
```

会自动下载、校验 sha256、原子替换二进制，并在 systemd 服务 active 时重启它。
注意 systemd unit 是只读文件系统（`ProtectSystem=strict`），**要在交互 shell 用
`sudo` 执行**，不要通过服务进程触发。也可直接重跑安装脚本。

### 忘了 Web 登录密码怎么办？

- 安装脚本部署：密码在首次启动日志里，`journalctl -u skyfire | grep password`；
  或用 `SKYFIRE_PASSWORD` 重装固定密码：
  ```bash
  sudo SKYFIRE_PASSWORD='new-pass' bash -c "$(curl -fsSL .../install.sh)"
  ```
- 手动运行：`-password '<new-pass>'` 重启即可覆盖。
- 也可以直接删掉登录会话（重启进程）后再用新密码登录。

### 打开 Web UI 是 401 / 一直让登录？

默认单用户密码认证。用 `-username`（默认 `admin`）+ 启动日志里的密码登录。
脚本/curl 可用 Bearer：`-H 'Authorization: Bearer <token>'`。

### API 全部返回 401，但我没设密码？

只要有 `-password`（默认会自动生成）或 `-token`，就要认证。两者都为空才是开放模式。

### 局域网内其他机器访问不到 Web UI？

- 确认监听地址是 `:51821`（绑定所有网卡）而非 `127.0.0.1:51821`。
- 检查防火墙放行 51821/tcp。
- systemd 部署确认服务在跑：`systemctl status skyfire`。

### 客户端配置里 `Endpoint = YOUR.SERVER.HOST.OR.IP`？

说明 `settings.publicEndpoint` 没配。到 Web UI「设置」里填服务器公网 IP/域名
（局域网测试填局域网 IP），保存后重新导出配置。注意填的是 **WireGuard 监听端口**
（默认 51820），不是 Web 的 51821。

### 创建接口报 "listen port already in use" / "already exists"？

每个接口的 `listenPort` 必须唯一，接口名也必须唯一。换一个端口或名称。

### 提示 "... (configuration was saved)" 是否成功？

这句表示**期望状态已写入配置文件**，但**下发到驱动的实时应用失败**。常见原因：
没有 root / `CAP_NET_ADMIN`、内核 WireGuard 不可用、接口重名、路由冲突。
查看 `journalctl -u skyfire -n 100` 定位。

### 升级后行为变了 / 转发不通？

默认驱动已切到 `netstack`（全用户态，**不做内核转发/NAT、不改主机路由**）。
若你依赖旧的系统栈行为，请显式 `-driver userspace`（或 `kernel`）重启。

### `-dry-run` 和 `-demo` 的区别？

- `-dry-run`：真实驱动初始化，但**只打印不执行**系统变更。
- `-demo`：`mock` 驱动 + `-dry-run` + 临时配置 + 账号 `admin/demo` + token `demo`，
  完全不碰系统，用于演示/熟悉 UI。

### 数据/私钥怎么备份和迁移？

全部状态在配置文件（默认 `/etc/skyfire/config.json`），**包含所有私钥和
Peer token**。停服务后拷贝该文件到新机器同路径即可；请按密钥保管（`chmod 600`）。

### systemd 下写文件失败 / ProtectSystem 报错？

unit 使用 `ProtectSystem=strict` + `ReadWritePaths=/etc/skyfire`，配置必须写在
`/etc/skyfire` 下。自定义路径时同步修改 `ReadWritePaths`。

---

## 客户端

### 客户端能通过局域网被"配置"吗？

不能远程推送。客户端是纯出站进程，没有监听端口。配置只能在本机通过
`-connect`、托盘菜单「Set connection string…」或本地 `-conf` 文件设置。
但它**可以**通过局域网从服务端拉配置——把连接字符串的主机换成局域网 IP 即可。

### 客户端能同时连多个服务端 / 开多个隧道吗？

不支持。一个进程只维护一个隧道；多实例会因固定接口名（`skyfire`/`Skyfire`）、
共享配置文件和共享路由表 `51821` 而冲突。

### `-connect` 会立即连接吗？

会。`skyfire-client -connect '<URL>'` 保存连接字符串并立即建立隧道（托盘启动即
已连接）。首次运行（尚无连接字符串）则弹框输入后自动连接。若只想保存连接字符串，
可不带 `-connect` 启动，再用托盘菜单设置。

### 报 "connection string is not a Skyfire peer config URL"？

连接字符串必须包含 `/api/p/`，且是 `http://` 或 `https://` 开头。请从 Web UI 的
Peer 详情「Desktop client」页签复制，不要手拼。

### `-dry-run` 说配置有效，但真实连接失败？

`-dry-run` 只验证配置**内容**，不建隧道。真实连接还依赖：管理员权限、
Wintun 驱动（Windows）、endpoint 可达、`wintun.dll` 在位等。

### Windows 报错找不到 Wintun / 无法创建网卡？

- 把 `wintun.dll` 放在 `skyfire-client.exe` **同一目录**。
- 以**管理员身份**运行（创建 Wintun 网卡需要权限）。

### macOS 提示"未公证 / 无法验证开发者"？

发布产物未签名/未公证。到「系统设置 → 隐私与安全性」允许后重开。

### 全隧道下网络时通时断 / 客户端自己失联？

这是**已知限制**：endpoint 例外路由尚未实现，全隧道（`AllowedIPs=0.0.0.0/0`）
下端点流量可能被误送进隧道。改用**分隧道**验证：把 Peer 的 `clientRoutes`
只填内网网段（如 `10.42.0.0/24`），不要用 `0.0.0.0/0`。

### DNS 配置没生效？

客户端的 DNS 处理因平台而异：**Windows** 会通过 `netsh` 设置接口 DNS 并在断开
时恢复 DHCP；**Linux / macOS 不修改**系统 DNS。需要时自行配置。

### 服务端临时不可达，客户端还能起吗？

能。客户端会缓存最近一次成功拉取的配置（`wg.conf`），拉取失败时自动回退使用并
打 warn 日志。

### 客户端配置/状态存在哪？

`<UserConfigDir>/skyfire-client/`：`config.json`（连接字符串）+ `wg.conf`（缓存）。
各平台路径见[客户端文档](client.md#4-命令行参数)。

---

## 相关文档

- [服务端 skyfired](server.md)
- [客户端 skyfire-client](client.md)
- [REST API](api.md)
