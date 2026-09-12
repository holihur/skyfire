# Skyfire

WireGuard 可视化配置管理工具：一个守护进程管理多个 WireGuard 接口与 Peer，
内置 Web UI（单用户登录），前端被打包进单个二进制。

- **后端**：Go + gVisor netstack（全用户态驱动，默认）/ `wireguard-go`（userspace 驱动）/ `wgctrl`（kernel 驱动）
- **前端**：React + Vite + Tailwind + Radix UI
- **安全**：默认不对系统做任何变更，可用 `-dry-run` 预览所有操作

## 特性

- 接口与 Peer 的增删改查，自动分配隧道地址
- 客户端配置下载（.conf 文本）+ 二维码图片
- 桌面客户端（单文件 + 系统托盘）凭 Peer 令牌一键连接
- 实时状态：连接状态、最后一次握手、收发流量
- 服务器内置 DNS 转发（客户端默认经隧道使用干净解析）+ 全局流量转发开关
- 单用户密码登录（Session Cookie）+ 可选 Bearer Token
- 单一可执行文件（Web UI 内嵌）

## 文档

- [服务端 skyfired 使用文档](docs/server.md)：安装、运行、配置、驱动、Web UI 操作
- [桌面客户端 skyfire-client 使用文档](docs/client.md)：平台、用法、路由、权限、限制
- [REST API](docs/api.md)：认证、端点、请求/响应示例
- [常见问题 / 故障排查](docs/faq.md)

## 快速安装（纯二进制，无需 Go/Node 工具链）

```bash
curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh | sudo bash
```

脚本从 GitHub Release 下载对应平台的预编译二进制（Linux 下同时可选装 systemd
服务）。固定版本：

```bash
sudo SKYFIRE_VERSION=v0.1.0 bash -c "$(curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh)"
# 固定登录密码（写入 /etc/skyfire/skyfire.env，不落进 unit 文件）：
sudo SKYFIRE_PASSWORD='your-password' bash -c "$(curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh)"
# Windows (git-bash):  可选安装后运行 skyfired.exe -demo / skyfired.exe
```

- Linux：`/usr/local/bin/skyfired`，自动生成 systemd 服务（监听 `:51821`），
  登录密码打印在 `journalctl -u skyfire -n 40`，可用 `-password` 固定
- Windows：`skyfired.exe`；真实隧道需要 Wintun 驱动，`-demo` 模式无需任何特权
- Windows (arm64)：无发布产物（goreleaser 跳过该目标）
- Windows 服务自启：暂不支持，需手动运行 `skyfired.exe`
- 从源码构建见下方“开发”（需要 Go ≥ 1.26、Node ≥ 20、pnpm）

### 更新

已安装后，用自更新命令升级到最新发布（校验 checksum + 原子替换，systemd
下自动重启服务）：

```bash
sudo skyfire update          # 等同 skyfired update
skyfire update -check        # 只检查是否有新版本
sudo skyfire update -version v0.6.0
```

安装脚本会额外装一个 `skyfire` 别名指向 `skyfired`，因此 `skyfire update` 与
`skyfired update` 等价。详见[服务端文档](docs/server.md#41-update-子命令)。

## 桌面客户端（一键连接）

单文件客户端 `skyfire-client`：内嵌 `wireguard-go`，系统托盘一键连接/断开，
凭连接字符串从服务端取配置，无需安装 WireGuard 官方客户端。

- **Windows**：系统托盘（纯 Go，无需 cgo）。发布产物
  `skyfire-client-windows-amd64.exe`；真实隧道需要 `wintun.dll`（放在 exe 同目录）
- **macOS**：系统托盘（需 cgo/Cocoa，由 CI 在 macOS 上构建）。发布产物
  `skyfire-client-darwin-amd64` / `-arm64`；未公证，首次运行需在“系统设置 →
  隐私与安全性”中放行
- **Linux**：可编译，但只提供终端模式（实验性，用于本地测试）

### 用法

在 Web UI 的 Peer 详情页「桌面客户端」页签复制**连接字符串**，或直接复制
**一键连接命令**发给终端用户。

```bash
# 保存连接字符串并立即连接（托盘启动即已连接；首次运行弹框输入）
skyfire-client -connect 'https://vpn.example.com:51821/api/p/<token>/wg.conf'

# 仅拉取并校验配置，不改动系统
skyfire-client -dry-run

# 无托盘环境用终端模式；也可用本地 .conf 而不从服务端拉取
skyfire-client -cli -conf ./peer.conf
```

托盘菜单：连接/断开、设置连接字符串、退出；图标颜色表示状态（绿=已连接）。
连接字符串也可只保存不用：不带 `-connect` 启动后由托盘菜单设置。

### 权限与路由说明

- 全隧道（`AllowedIPs` 含 `0.0.0.0/0`）会安装默认路由：Linux 用 wg-quick 风格
  策略路由（fwmark + 独立路由表 `51821`，避免与 wg-quick 的 `51820` 冲突）；
  Windows/macOS 用两条 `/1` 路由。
- Windows 真实隧道首次需管理员权限（创建 Wintun 网卡）；macOS 无公证会被
  Gatekeeper 拦截。
- 全隧道防环：Linux 用 fwmark 策略路由；Windows/macOS 在装载 `/1` 默认路由前，
  先把每个 endpoint 钉到物理默认网关（`/32` 例外路由），避免隧道自身流量被
  卷进隧道造成路由死循环/断网。

## 手动运行

> 完整参数、驱动说明与 Web UI 操作见 [服务端文档](docs/server.md)。

```bash
make demo   # 安全演示模式：mock 驱动 + 示例数据，不碰真实网络
make        # 构建前端并打包进二进制
sudo ./skyfired -config /etc/skyfire/config.json -addr :51821
```

### 参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-config` | `/etc/skyfire/config.json` | 持久化配置文件 |
| `-addr` | `:51821` | HTTP 监听地址 |
| `-driver` | `netstack` | `netstack`（默认，全用户态） \| `userspace` \| `kernel` \| `mock`。升级时若需保持旧行为，显式传 `-driver userspace` |
| `-dry-run` | `false` | 只打印、不执行任何系统变更（安全预览） |
| `-token` | 空 | Bearer Token（空则仅用密码登录） |
| `-username` | `admin` | 登录用户名 |
| `-password` | 自动生成 | 登录密码 |
| `-static` | 空（内嵌 UI） | 指定前端目录，覆盖内嵌版本 |
| `-dns` | `127.0.0.1:53` | 隧道 DNS 转发器监听地址（空禁用） |
| `-dns-upstream` | `8.8.8.8:53,1.1.1.1:53` | DNS 转发器上游 |
| `-demo` | `false` | mock 驱动 + 示例数据，打印密码 `demo` |

## 开发

```bash
# 后端
cd backend && go test ./... && go vet ./...

# 桌面客户端
cd client && go vet ./... && go build ./...

# 前端（热更新开发）
cd frontend && pnpm install && pnpm dev
```

## API

REST API 位于 `/api`，认证方式：`Authorization: Bearer <token>` 或登录后的
Session Cookie。完整文档（含请求/响应示例）见 [docs/api.md](docs/api.md)。
主要端点：

- `GET/PUT /api/settings` — 全局设置（公网端点等）
- `GET/POST /api/interfaces` — 接口列表 / 创建
- `GET/PUT/DELETE /api/interfaces/{name}` — 单个接口
- `POST /api/interfaces/{name}/up` — 启停
- `GET /api/interfaces/{name}/config` — 服务端 wg-quick 配置
- `POST /api/interfaces/{name}/peers` — 添加 Peer
- `GET /api/interfaces/{name}/peers/{key}/config[.png]` — 客户端配置 / QR 码
- `GET /api/p/{token}/wg.conf` — 凭 Peer 令牌获取客户端配置（免登录，仅此 Peer）
- `GET /api/p/{token}/wg.png` — 同上，二维码图片

## 许可证

MIT