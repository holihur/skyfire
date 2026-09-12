# Skyfire 服务端（skyfired）使用文档

`skyfired` 是 Skyfire 的守护进程：它把一份持久化的"期望状态"（接口 + Peer）
下发到一个 WireGuard 驱动，并对外提供 JSON REST API 与内嵌 Web UI。一个进程
可以同时管理**多个** WireGuard 接口。

- 服务端在**默认 `netstack` 驱动**下是全用户态实现，不依赖内核 WireGuard、
  也不需要 iptables/NAT；只有 `kernel` / `userspace` 驱动才会改动主机的路由与
  转发（见「驱动」一节）。
- 首次启动会随机生成一个 Web 登录密码并打印到日志。

---

## 1. 安装

### 1.1 一键脚本（推荐，无需 Go/Node）

```bash
curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh | sudo bash
```

脚本会：下载当前平台（linux/darwin/windows；amd64/arm64，其中 Windows arm64
无发布产物）的预编译二进制、校验 `checksums.txt`、安装到 `$PREFIX/bin/skyfired`；
在 **Linux + systemd** 环境下还会自动写并启动 `skyfire.service`。

可用环境变量：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `SKYFIRE_VERSION` | `latest` | 固定发布版本，如 `v0.1.0` |
| `SKYFIRE_PASSWORD` | 空（每次启动随机） | 固定 Web 登录密码，写入 `/etc/skyfire/skyfire.env`（`chmod 600`，不落进 unit 文件） |
| `PREFIX` | `/usr/local` | 安装前缀 |
| `ETC` | `/etc/skyfire` | 配置目录（Linux） |

示例：

```bash
# 固定版本
sudo SKYFIRE_VERSION=v0.1.0 bash -c \
  "$(curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh)"

# 固定密码（不打印、可重复安装）
sudo SKYFIRE_PASSWORD='your-password' bash -c \
  "$(curl -fsSL https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install.sh)"
```

> 脚本可重复执行：它会 `systemctl restart skyfire`（而非 `--now`），
> 因此升级时可以放心重跑以替换二进制。

### 1.2 从源码构建

```bash
make          # 构建前端并打包进 skyfired（会用到 pnpm）
sudo make install   # 安装二进制 + deploy/skyfire.service
```

需要 Go ≥ 1.26、Node ≥ 20、pnpm。

---

## 2. 运行

### 2.1 演示模式（安全，无需 root）

```bash
make demo
# 等价于：./skyfired -demo -addr :51821
```

`-demo` 会强制：`mock` 驱动 + `dry-run` + 配置文件放临时目录 +
账号 `admin` / 密码 `demo` + Bearer token `demo`。不会对系统做任何改动，
适合先熟悉 Web UI。

### 2.2 手动运行（真实隧道）

```bash
# 真实网络变更需要 root / CAP_NET_ADMIN
sudo ./skyfired -config /etc/skyfire/config.json -addr :51821

# 只看不改：打印所有会执行的操作
sudo ./skyfired -dry-run -driver kernel -addr :51821

# 固定密码
sudo ./skyfired -password 'your-password'
```

### 2.3 systemd（安装脚本自动完成）

```bash
systemctl status skyfire
journalctl -u skyfire -n 60        # 首次启动的随机密码在这里
systemctl restart skyfire
```

生成的 unit 关键点：

```ini
ExecStart=/usr/local/bin/skyfired -config /etc/skyfire/config.json -addr :51821
EnvironmentFile=-/etc/skyfire/skyfire.env     # 可选 SKYFIRE_PASSWORD
ProtectSystem=strict
ReadWritePaths=/etc/skyfire
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
Restart=on-failure
```

> 仓库里另有 `deploy/skyfire.service`（供 `make install` 使用），
> 与安装脚本生成的版本基本一致，只是没有 `EnvironmentFile` 的 `-password`。

---

## 3. 配置文件与路径解析

`-config` 为空时按顺序解析：

1. `/etc/skyfire/config.json`（若目录可写，root 下即此）
2. `$XDG_CONFIG_HOME/skyfire/config.json`
3. `~/.config/skyfire/config.json`
4. 当前目录的 `skyfire.json`

配置文件是 JSON 期望状态，**包含所有私钥**，请按密钥对待（`chmod 600`）。
结构：

```json
{
  "version": 1,
  "settings": { "publicEndpoint": "vpn.example.com" },
  "interfaces": [
    {
      "name": "wg0",
      "publicKey": "...",
      "privateKey": "...",
      "listenPort": 51820,
      "addresses": ["10.42.0.1/24"],
      "mtu": 1420,
      "dns": ["1.1.1.1"],
      "up": true,
      "peers": [
        {
          "name": "laptop",
          "publicKey": "...",
          "privateKey": "...",
          "clientToken": "...",
          "address": "10.42.0.2",
          "allowedIPs": ["10.42.0.2/32"],
          "clientRoutes": ["0.0.0.0/0", "::/0"],
          "dns": ["1.1.1.1"],
          "persistentKeepalive": 25,
          "enabled": true
        }
      ]
    }
  ]
}
```

直接用 API/Web UI 修改即可，无需手改文件。

---

## 4. 命令行参数

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-config` | 自动解析 | 持久化配置文件路径 |
| `-addr` | `:51821` | HTTP 监听地址（`:51821` = 所有网卡） |
| `-driver` | `netstack` | `netstack` \| `userspace` \| `kernel` \| `mock` |
| `-dry-run` | `false` | 只记录、不执行任何系统变更 |
| `-token` | 空 | Bearer Token（空则关闭 token 认证） |
| `-username` | `admin` | 单用户 Web 登录名 |
| `-password` | 自动生成 | 单用户 Web 登录密码 |
| `-static` | 空 | 指定前端目录，覆盖内嵌 UI |
| `-dns` | `127.0.0.1:53` | 隧道 DNS 转发器监听地址（空 = 禁用；`-dry-run`/`-demo` 下自动禁用） |
| `-dns-upstream` | `8.8.8.8:53,1.1.1.1:53` | 转发器上游 DNS（逗号分隔，逐个尝试） |
| `-demo` | `false` | mock 驱动 + 示例数据 + `dry-run` |
| `-verbose` | `false` | debug 日志 |
| `-version` | `false` | 打印版本后退出 |

### 4.1 update 子命令

`skyfire update`（等同 `skyfired update`）从 GitHub Release 自更新：解析最新
（或指定）发布 → 下载当前平台二进制 → 用 `checksums.txt` 校验 sha256 →
原子替换自身 → 若作为 systemd 服务运行则自动重启。

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-check` | `false` | 只检查是否有新版本，不安装 |
| `-version` | 空 | 安装指定 tag（如 `v0.6.0`；空 = 最新） |
| `-force` | `false` | 当前版本不比最新旧时也强制安装 |
| `-restart` | `true` | 更新后重启 `skyfire` systemd 服务 |
| `-repo` | `holihur/skyfire` | GitHub 仓库（owner/name） |
| `-api` | `https://api.github.com` | GitHub API 基址（镜像/测试用） |

```bash
# 检查是否有新版本
skyfire update -check

# 更新到最新（需写权限，通常加 sudo）
sudo skyfire update

# 固定到某个版本 / 不重启服务
sudo skyfire update -version v0.6.0
sudo skyfired update -restart=false
```

> 设置 `GITHUB_TOKEN` 环境变量可提高 GitHub API 速率限制。
> 源码构建的二进制版本为 `dev`，无法与发布版本比较，会直接安装最新版。

### 4.2 隧道 DNS 转发器（-dns / -dns-upstream）

skyfired 内置一个**无解析的透明 DNS 转发器**：监听 `:53`（UDP+TCP），把客户端的
DNS 请求原样转发到 `-dns-upstream` 指定的干净上游（默认 `8.8.8.8`、`1.1.1.1`）。

工作机制：

- 客户端 `wg.conf` 的 `DNS` 默认为**服务器隧道地址**（接口 `addresses` 的第一个
  IPv4，如 `10.42.0.1`）；每 Peer 可在 `dns` 字段覆盖。
- 客户端把 DNS 发到 `10.42.0.1:53`；netstack 驱动会自动把它转到本机回环，命中
  转发器（`userspace`/`kernel` 驱动走系统栈，同样可达）。
- 转发器通过干净上游解析，**绕过客户端本地被污染的 DNS**（如 GFW 污染），且对
  全隧道、分隧道都有效。

```bash
# 默认：监听 :53，转发到 8.8.8.8 / 1.1.1.1
sudo skyfired -config /etc/skyfire/config.json -addr :51821

# 自定义上游 / 端口 / 禁用
sudo skyfired -dns :53 -dns-upstream 1.1.1.1:53,9.9.9.9:53 ...
sudo skyfired -dns '' ...   # 禁用转发器
```

> 监听 `:53` 需要 root；端口被占用时仅打 warn，不会退出。客户端连接时会应用
> `wg.conf` 的 `DNS`（Windows `netsh`、Linux `resolvectl`/`resolvconf`、
> macOS `scutil`；断开时还原）。

---

## 5. 驱动（-driver）

| 驱动 | 内核转发/NAT | 说明 |
| --- | --- | --- |
| `netstack`（默认） | 否（全用户态） | 使用 gVisor netstack，一个进程内实现转发；**不改动主机路由、转发或 NAT**，最安全 |
| `userspace` | 是 | 内嵌 `wireguard-go` + OS 路由/转发，接近 wg-quick 行为 |
| `kernel` | 是 | 通过 wgctrl 操作内核 WireGuard 接口 |
| `mock` | 否 | 无副作用，供 `-demo` / 测试使用 |

升级提示：若你此前依赖 `userspace` 的行为，升级后请显式加 `-driver userspace`。

---

## 6. 认证与安全

- **登录密码**：单用户、Session Cookie（`skyfire_session`，HttpOnly，TTL 24h）。
  密码为空时每次启动随机生成一个 16 位密码并打印到日志；用 `-password`
  或 `SKYFIRE_PASSWORD` 固定。
- **Bearer Token**：`Authorization: Bearer <token>`，适合脚本/非浏览器调用。
- 两者都为空 ⇒ API **完全无认证**（仅建议本地/演示环境），启动时会打 warn。
- **始终免认证**的端点：
  - `POST /api/login`、`POST /api/logout`
  - `GET /api/p/{token}/wg.conf`、`GET /api/p/{token}/wg.png`
    （token 本身即凭据，且只暴露该 Peer 的配置）
- 其余一切 `/api/*` 都需要认证。
- 建议：公网部署时置于 HTTPS 反向代理之后；`/api/p/` 的 token 是明文凭据，
  走 TLS 更安全。

---

## 7. Web UI 操作指南

首次打开 `http://<host>:51821` 用 `admin` + 密码登录。页面：

- `/`（Dashboard）：接口卡片列表、连接状态、流量、设置入口。
- `/interfaces/:name`（InterfaceDetail）：单个接口的 Peer 表与操作。

典型流程：

1. **设置公网端点**：Dashboard → 设置（Settings），填 `publicEndpoint`
   （见第 8 节）。局域网测试填服务器局域网 IP。
2. **创建接口**：New Interface → 填 `name`（小写字母/数字/`-`/`_`，≤15 字符）、
   `listenPort`、`addresses`（CIDR，如 `10.42.0.1/24`）、可选 `mtu`/`dns`、
   `up`。私钥留空会自动生成。
3. **启停接口**：接口卡片上的 Up/Down 开关。
4. **添加 Peer**：接口详情 → Add Peer → 填 `name`、`address`（留空自动分配）、
   `clientRoutes`（推送给客户端的 AllowedIPs，默认 `0.0.0.0/0, ::/0`）、
   可选 PSK、`persistentKeepalive`。勾选 generate keys 自动生成密钥对。
5. **导出配置**：Peer 详情弹窗有四个页签：
   - **QR**：配置二维码（手机/平板扫码导入）
   - **Conf**：客户端 `.conf` 文本，可复制/下载
   - **Desktop client**：**连接字符串**（`{origin}/api/p/{token}/wg.conf`）与
     **一键连接命令**（`skyfire-client -connect '<URL>'`，可选 Linux/macOS 或
     Windows），复制给终端用户即可直接执行连接
   - **Info**：地址、公钥、握手时间、收发流量
6. **导出服务端配置**：接口详情可下载服务端 wg-quick 配置、查看/复制私钥。

---

## 8. 全局设置：PublicEndpoint

`settings.publicEndpoint` 会被写进每个客户端配置的 `[Peer] Endpoint`：

- 可以是纯主机名/IP，也可以是 `host:port`。
- 若是纯主机/IP，会自动拼接该接口的 `listenPort`。
- 为空时生成占位符 `YOUR.SERVER.HOST.OR.IP`，客户端**连不上**。

```
公网：   publicEndpoint = vpn.example.com
局域网： publicEndpoint = 192.168.1.10
带端口： publicEndpoint = vpn.example.com:51820
```

> 注意：这是 **WireGuard 监听端口**（接口的 `listenPort`，默认 51820），
> 不是 Web UI 的 51821。

### 流量转发（forwarding）

`settings.forwarding`（Web UI：设置 → 通用 → 流量转发）控制是否允许隧道客户端
经本服务器访问外部网络（互联网中转）。默认**开启**（字段缺省即开启）。

- **开启**：客户端可经服务器上网（全隧道 / NAT）。
- **关闭**：客户端只能访问隧道内地址与服务器本身，公网中转流量被丢弃；隧道
  DNS（服务器隧道地址）不受影响，仍可用。

改动会**即时热应用**到已启用的接口，无需重连。注意：关闭转发后，从**隧道内**
用公网 IP 访问 Web API 也会被当作中转而拒绝——请改用服务器隧道地址（如
`http://10.42.0.1:51821`）或本地/直连方式管理。

---

## 9. 升级 / 卸载

### 升级：用自更新命令

```bash
# 等价写法：skyfired update
sudo skyfire update
```

它会下载最新发布、校验 checksum、替换二进制，并在检测到 `skyfire`
systemd 服务处于 active 时自动重启。也可用安装脚本重跑（同样会重启服务）：

```bash
curl -fsSL .../install.sh | sudo bash
```

> systemd unit 以 `ProtectSystem=strict` 运行，**无法**自行写入 `/usr/local/bin`；
> 自更新请在交互 shell 里用 `sudo skyfire update` 执行，不要通过服务进程触发。

### 卸载（Linux systemd）

```bash
sudo systemctl disable --now skyfire
sudo rm -f /etc/systemd/system/skyfire.service
sudo systemctl daemon-reload
sudo rm -f /usr/local/bin/skyfired /usr/local/bin/skyfire
sudo rm -rf /etc/skyfire        # 含所有私钥，谨慎
```

---

## 10. 相关文档

- [客户端 skyfire-client](client.md)
- [REST API](api.md)
- [常见问题 / 故障排查](faq.md)
