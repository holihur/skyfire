# Skyfire 桌面客户端（skyfire-client）使用文档

`skyfire-client` 是单文件桌面客户端：内嵌 `wireguard-go`，用系统托盘（或终端）
一键连接/断开，凭**连接字符串**从服务端拉取配置，无需安装 WireGuard 官方客户端。

- 工作方式：**只做出站请求**。向连接字符串对应的 URL 拉取本次 Peer 的
  `wg.conf`，在本地建用户态隧道；没有监听端口，也不会被局域网远程配置。
- 一个进程同一时刻只维护**一个**隧道。

---

## 1. 平台与产物

| 平台 | 产物 | 界面 | 依赖 |
| --- | --- | --- | --- |
| Windows amd64 | `skyfire-client-windows-amd64.exe` | 系统托盘 + fyne 对话框（**需 cgo/mingw 构建**） | 真实隧道需 `wintun.dll` 与 exe 同目录 |
| macOS amd64/arm64 | `skyfire-client-darwin-amd64` / `-arm64` | 系统托盘 + fyne 对话框（需 cgo/Cocoa，CI 在 macOS 构建） | 未公证，首次运行需在「系统设置 → 隐私与安全性」放行 |
| Linux | 可自行编译 | **仅终端模式**（命令行） | — |

> Windows arm64 无发布产物（goreleaser 跳过该目标）。
>
> 桌面界面（托盘 + 对话框）与多语言基于 [fyne](https://fyne.io)，仅用于
> **Windows 与 macOS**，因此这两个平台**需要 cgo**：Windows 交叉编译要
> mingw-w64，macOS 要 Xcode。Linux 始终是纯 Go 的命令行客户端。

---

## 2. 获取连接字符串

在服务端 Web UI：接口详情 → Peer 详情弹窗 → **Desktop client** 页签 → 复制
**连接字符串**（也可直接复制下方的**一键连接命令**给终端用户）。它形如：

```
https://vpn.example.com:51821/api/p/<token>/wg.conf
```

- 该 URL 用 Peer 的 `clientToken` 鉴权，**免登录**，且**只暴露这一个 Peer**。
- 局域网测试时可把主机换成服务器局域网 IP，例如
  `http://192.168.1.10:51821/api/p/<token>/wg.conf`（协议 `http` 亦可）。
- 连接字符串必须包含 `/api/p/`，否则客户端拒绝保存。

> 想让隧道真正连通，服务端的 `settings.publicEndpoint` 必须是客户端可达的
> 地址；详见 [服务端文档 · PublicEndpoint](server.md#8-全局设置publicendpoint)。

---

## 3. 安装 / 构建

### 3.1 下载发布产物

从 GitHub Release 下载对应平台文件，Windows 记得把 `wintun.dll` 放在同一目录。

### 3.2 从源码构建

```bash
# 当前平台（Windows/macOS 为 fyne 托盘 + 对话框；Linux 为终端版）
make client

# Windows amd64（fyne 托盘 + 对话框；交叉编译需要 mingw-w64）
sudo apt-get install gcc-mingw-w64-x86-64
make client-windows

# macOS arm64（fyne 托盘需 cgo/Cocoa，在 macOS 上构建）
make client-darwin

# Linux 终端版（无 cgo）
make client-cli
```

产物输出到仓库根目录。手动构建示例：

```bash
cd client
# Windows（需要 mingw-w64）
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o skyfire-client.exe .
# macOS（fyne 托盘，在 macOS 上）
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o skyfire-client .
# Linux 终端版（无 cgo）
CGO_ENABLED=0 go build -o skyfire-client .
```

---

## 4. 命令行参数

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-connect` | 空 | 连接字符串；保存后**立即连接**（托盘启动即已连接） |
| `-conf` | 空 | 使用本地 WireGuard `.conf` 文件，改为从文件而非服务端拉取 |
| `-whitelist` | 空 | 逗号分隔的域名/CIDR 白名单，仅这些目标走隧道；显式传空则清空（恢复全隧道） |
| `-lang` | 空 | 界面语言：`en` 或 `zh`；留空则用已保存设置，再否则自动检测系统语言 |
| `-no-reconnect` | `false` | 关闭掉线自动重连（默认开启） |
| `-dry-run` | `false` | 只拉取并校验配置，不建隧道、不改系统 |
| `-cli` | `false` | 终端模式而非系统托盘 |
| `-verbose` | `false` | debug 日志 |
| `-version` | `false` | 打印版本后退出 |

配置文件存放在用户配置目录：

- Linux：`~/.config/skyfire-client/config.json`
- macOS：`~/Library/Application Support/skyfire-client/config.json`
- Windows：`%AppData%\skyfire-client\config.json`

同目录还会缓存最近一次成功拉取的配置（`wg.conf`），服务端临时不可达时自动回退使用。

### 多语言

桌面界面支持 **English / 中文**，来源优先级：`-lang` 参数 > 已保存的 `lang` 配置 >
系统语言（Windows 读系统区域，其他平台读 `LANG`/`LC_ALL`/`SKYFIRE_LANG`）。
托盘右键菜单里有 **Language** 子菜单可随时切换，选择会写入配置。

---

## 5. 典型用法

```bash
# ① 校验配置（推荐首次先跑）：拉取 + 解析，不改系统
skyfire-client -connect 'https://vpn.example.com:51821/api/p/<token>/wg.conf' -dry-run

# ② 首次连接（托盘）：首次运行会弹框要求输入连接字符串并自动连接
skyfire-client -connect 'https://vpn.example.com:51821/api/p/<token>/wg.conf'

# ③ 无托盘环境用终端模式（Linux 等），启动即连接，Ctrl+C 断开退出
skyfire-client -cli -connect 'https://vpn.example.com:51821/api/p/<token>/wg.conf'

# ④ 完全离线：用本地 .conf，不从服务端拉取
skyfire-client -cli -conf ./peer.conf

# ⑤ 只保存连接字符串，之后再从托盘菜单连（不带 -connect 启动）
skyfire-client
```

行为细节：

- **托盘模式**：`-connect` 会保存连接字符串并**立即连接**（托盘启动即为已连接
  状态）。首次运行（尚无连接字符串）会弹框要求输入并自动连接。
- **终端模式（`-cli`）**：启动即连接，阻塞直到 `Ctrl+C` / `SIGTERM`，然后断开。
- **`-dry-run`**：解析并打印地址、DNS、MTU、Peer、是否全隧道，然后退出。
- **来源优先级**：`-conf` 本地文件 > 连接字符串拉取 > 磁盘缓存。

---

## 6. 托盘菜单

- **状态行**：Disconnected / Connecting / Connected / Error。
- **Connect / Disconnect**：切换隧道。
- **Set connection string…**：输入/更新连接字符串（Windows 用原生输入框，
  macOS 用 osascript 对话框）。
- **Quit**：断开并退出。

图标颜色表示状态（绿=已连接）。连接字符串无效时日志会给出提示。

---

## 7. 路由、DNS 与权限

全隧道判定：任一 Peer 的 `AllowedIPs` 含 `0.0.0.0/0` 或 `::/0`。

| 平台 | 全隧道默认路由 | 端点防环 | DNS |
| --- | --- | --- | --- |
| Linux | wg-quick 风格策略路由：fwmark + 独立路由表 `51821`（避开 wg-quick 的 51820） | 用 fwmark 规则排除端点自身流量 | 连接时用 `resolvectl`（systemd-resolved）/ `resolvconf` / 直接改 `/etc/resolv.conf` 指向隧道 DNS，断开还原 |
| Windows | 两条 `/1` 路由（`0.0.0.0/1` + `128.0.0.0/1`） | 装默认路由前把 endpoint 钉到物理默认网关（`route add <ep>/32 <gw>`），断开删除 | 通过 `netsh` 写接口 DNS，断开时恢复 DHCP |
| macOS | 两条 `/1` 路由 | 装默认路由前把 endpoint 钉到物理默认网关（`route add -host <ep> <gw>`），断开删除 | 通过 `scutil` 安装解析器，断开时移除 |

客户端配置里的 `DNS` 字段：服务端默认下发**服务器隧道地址**（接口 `addresses`
的第一个 IPv4，如 `10.42.0.1`，由 skyfired 内置 DNS 转发器应答）；Peer 显式
设置的 `dns` 优先。客户端连接时会应用该 DNS（Linux 优先 `resolvectl`，其次
`resolvconf`，最后直接改 `/etc/resolv.conf` 并备份；macOS 用 `scutil`），断开
时全部还原。

权限：

- Windows 真实隧道首次需**管理员权限**（创建 Wintun 网卡）。
- macOS 未公证会被 Gatekeeper 拦截，需手动放行；建 utun 通常也需相应权限。

> **端点防环**：全隧道时 Windows/macOS 会在安装 `/1` 默认路由前，解析每个
> Peer 的 `Endpoint` 并加一条 `/32` 例外路由指向当前物理默认网关，断开时删除。
> 若无法解析 endpoint 或找不到默认网关，客户端会**拒绝建立全隧道**（而不是冒
> 断网风险）。endpoint 为 IPv6 时例外路由暂未覆盖。

### 7.1 域名 / CIDR 白名单（按域名分流）

配置白名单后客户端进入**白名单模式**：只有白名单里的目标走隧道，其余流量走
物理网络直连（服务端配置的 `AllowedIPs` 全隧道路由会被忽略）。白名单支持三类
条目，可混写、逗号分隔：

| 条目 | 示例 | 行为 |
| --- | --- | --- |
| 精确域名 | `api.example.com` | 解析该域名，把返回的 IP 走隧道 |
| 域名通配符 | `*.example.com` | 匹配任意子域名（含 `example.com`），按需解析并走隧道 |
| IP / CIDR | `10.0.0.0/8`、`192.168.1.5` | 直接加路由走隧道（裸 IP 视为 /32 或 /128） |

设置方式：

```bash
# 保存并立即生效（下次连接）
skyfire-client -connect '<connect-url>' -whitelist 'api.example.com,*.corp.example,10.0.0.0/8'

# 清空白名单（恢复服务端配置的全隧道 / 分流）
skyfire-client -whitelist ''
```

托盘菜单里也有 **Set domain whitelist…**，弹框输入即可（留空=清空）。白名单
保存在 `config.json` 的 `whitelist` 字段。

实现要点与限制：

- **CIDR**：连接时直接加路由，各平台均生效。
- **精确域名**：连接时通过隧道 DNS 解析并加主机路由，之后每 60 秒重新解析一次
  （CDN IP 变化会自动更新）。
- **域名通配符**：客户端在 `127.0.0.1:53` 起一个**本地 split-DNS 代理**，并把
  系统解析器指向它；匹配白名单的查询经隧道 DNS 解析并加路由，其余查询转发给
  系统原 DNS。解析失败时回退到系统原 DNS。
- 白名单域名通过**隧道 DNS**解析（服务端 `DNS` 字段），拿不到时才回退到系统
  DNS，以便内网 / 分域解析正确。
- 端口 `53` 被占用时代理无法启动，此时退化为「CIDR + 精确域名」可用、通配符
  失效（日志会提示）。
- Windows 上系统 DNS 覆盖依赖 Wintun 适配器 DNS 优先级，部分网络环境下通配符
  可能不生效；CIDR 与精确域名不受影响。

---

## 8. 多隧道说明

同一台机器**不支持**同时跑多个客户端实例：

- Linux `tunName()` 固定为 `skyfire`，Windows 固定为 `Skyfire`（Wintun 适配器名），
  第二个实例会因接口重名而失败。
- 所有实例共用同一个 `os.UserConfigDir()/skyfire-client/config.json`，会互相覆盖。
- Linux 全隧道共用同一 fwmark / 路由表 `51821`，会相互冲突。

需要连多个服务端时，请分机器、或对客户端做多连接改造。

---

## 9. 相关文档

- [服务端 skyfired](server.md)
- [REST API](api.md)
- [常见问题 / 故障排查](faq.md)
