# Skyfire

WireGuard 可视化配置管理工具：一个守护进程管理多个 WireGuard 接口与 Peer，
内置 Web UI（单用户登录），前端被打包进单个二进制。

- **后端**：Go + gVisor netstack（全用户态驱动，默认）/ `wireguard-go`（userspace 驱动）/ `wgctrl`（kernel 驱动）
- **前端**：React + Vite + Tailwind + Radix UI
- **安全**：默认不对系统做任何变更，可用 `-dry-run` 预览所有操作

## 特性

- 接口与 Peer 的增删改查，自动分配隧道地址
- 客户端配置下载（.conf 文本）+ 二维码图片
- 实时状态：连接状态、最后一次握手、收发流量
- 单用户密码登录（Session Cookie）+ 可选 Bearer Token
- 单一可执行文件（Web UI 内嵌）

## 快速安装（纯二进制，无需 Go/Node 工具链）

```bash
curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh | sudo bash
```

脚本从 GitHub Release 下载对应平台的预编译二进制（Linux 下同时可选装 systemd
服务）。固定版本：

```bash
sudo SKYFIRE_VERSION=v0.1.0 bash -c "$(curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh)"
# Windows (git-bash):  可选安装后运行 skyfired.exe -demo / skyfired.exe
```

- Linux：`/usr/local/bin/skyfired`，自动生成 systemd 服务（监听 `:51821`），
  登录密码打印在 `journalctl -u skyfire -n 40`，可用 `-password` 固定
- Windows：`skyfired.exe`；真实隧道需要 Wintun 驱动，`-demo` 模式无需任何特权
- Windows (arm64)：无发布产物（goreleaser 跳过该目标）
- Windows 服务自启：暂不支持，需手动运行 `skyfired.exe`
- 从源码构建见下方“开发”（需要 Go ≥ 1.26、Node ≥ 20、pnpm）

## 手动运行

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
| `-demo` | `false` | mock 驱动 + 示例数据，打印密码 `demo` |

## 开发

```bash
# 后端
cd backend && go test ./... && go vet ./...

# 前端（热更新开发）
cd frontend && pnpm install && pnpm dev
```

## API

REST API 位于 `/api`，认证方式：`Authorization: Bearer <token>` 或登录后的
Session Cookie。主要端点：

- `GET/PUT /api/settings` — 全局设置（公网端点等）
- `GET/POST /api/interfaces` — 接口列表 / 创建
- `GET/PUT/DELETE /api/interfaces/{name}` — 单个接口
- `POST /api/interfaces/{name}/up` — 启停
- `GET /api/interfaces/{name}/config` — 服务端 wg-quick 配置
- `POST /api/interfaces/{name}/peers` — 添加 Peer
- `GET /api/interfaces/{name}/peers/{key}/config[.png]` — 客户端配置 / QR 码

## 许可证

MIT