# Skyfire REST API

基址：`http://<host>:51821`。除个别端点外，`/api/*` 需要认证。

---

## 1. 认证

两种方式（满足其一即可）：

1. **Session Cookie**（浏览器）：
   ```bash
   curl -c cookie.txt -X POST http://localhost:51821/api/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"<password>"}'
   # 之后带 Cookie
   curl -b cookie.txt http://localhost:51821/api/interfaces
   ```
2. **Bearer Token**（脚本）：
   ```bash
   curl -H 'Authorization: Bearer <token>' http://localhost:51821/api/interfaces
   ```

- 若服务端 `-token` 与 `-password` 都为空 ⇒ API **完全无认证**。
- **始终免认证**：`POST /api/login`、`POST /api/logout`、`GET /api/auth`、
  `GET /api/p/{token}/wg.conf`、`GET /api/p/{token}/wg.png`。
- 未认证访问返回 `401 {"error":"unauthorized"}`。

---

## 2. 通用约定

- 请求体：JSON；解码时**拒绝未知字段**（多余字段 → `400`）。
- 成功：`200`/`201` + JSON，或 `204`（无内容，删除类）。
- 失败：`{"error":"<message>"}`，状态码 `400` 参数错 / `401` 未认证 /
  `404` 不存在 / `409` 冲突 / `500` 内部错误。
- **公钥路径编码**：WireGuard 公钥是标准 base64（含 `+`、`/`、`=`），
  不能直接放 URL。`/peers/{key}` 使用**无填充的 URL-safe base64**。
  转换可用：
  ```bash
  urlkey() { printf '%s' "$1" | tr '+/' '-_' | tr -d '='; }
  # 反向（服务端内部还原为标准 base64）
  ```

### 2.1 类型参考

**InterfaceView**

```json
{
  "name": "wg0",
  "publicKey": "…base64…",
  "listenPort": 51820,
  "addresses": ["10.42.0.1/24"],
  "mtu": 1420,
  "dns": ["1.1.1.1"],
  "up": true,
  "running": true,
  "dryRun": false,
  "totalPeers": 2,
  "connectedPeers": 1,
  "transferRx": 1234,
  "transferTx": 5678,
  "peers": [ /* PeerView[] */ ],
  "createdAt": "2025-01-02T03:04:05Z",
  "updatedAt": "2025-01-02T03:04:05Z"
}
```

**PeerView**

```json
{
  "name": "laptop",
  "publicKey": "…base64…",
  "presharedKey": "",
  "clientToken": "…opaque…",
  "address": "10.42.0.2",
  "allowedIPs": ["10.42.0.2/32"],
  "clientRoutes": ["0.0.0.0/0", "::/0"],
  "dns": ["1.1.1.1"],
  "endpoint": "",
  "persistentKeepalive": 25,
  "downloadLimit": 0,
  "uploadLimit": 0,
  "description": "",
  "enabled": true,
  "connected": false,
  "latestHandshake": "0001-01-01T00:00:00Z",
  "transferRx": 0,
  "transferTx": 0,
  "createdAt": "2025-01-02T03:04:05Z",
  "updatedAt": "2025-01-02T03:04:05Z"
}
```

**PeerInput**（创建/更新请求体，更新时整体覆盖）

```json
{
  "name": "laptop",
  "address": "10.42.0.2",
  "publicKey": "",
  "generateKeys": true,
  "presharedKey": "",
  "withPreshared": false,
  "allowedIPs": [],
  "clientRoutes": ["0.0.0.0/0", "::/0"],
  "dns": ["1.1.1.1"],
  "endpoint": "",
  "persistentKeepalive": 25,
  "downloadLimit": 0,
  "uploadLimit": 0,
  "description": "",
  "enabled": true
}
```

- `generateKeys=true` 或 `publicKey` 为空 ⇒ 服务端生成密钥对并保存私钥。
- `address` 为空 ⇒ 自动分配下一个空闲地址。
- `allowedIPs` 为空 ⇒ 默认取该地址的 `/32`（服务端侧可见源地址）。
- `clientRoutes` 为空 ⇒ 默认 `["0.0.0.0/0","::/0"]`（推送给客户端）。
- `withPreshared=true` 且 `presharedKey` 为空 ⇒ 自动生成 PSK。
- `downloadLimit`/`uploadLimit` 为按对端限速，单位 **bit/s**，`0` 表示不限速。
  `downloadLimit` 限制服务端 → 对端（对端下载），`uploadLimit` 限制对端 → 服务端
  （对端上传）。限速按对端的 `allowedIPs`（缺省为其隧道地址 /32 或 /128）匹配。
  仅 Linux 的 kernel / userspace 驱动通过 `tc` 生效；netstack 驱动在进程内中转层
  生效；其余平台/驱动会保留配置但不强制执行（日志会有提示）。

**InterfacePatch**（更新请求体，字段缺省=不变）

```json
{ "listenPort": 51821, "addresses": ["10.42.0.1/24"], "mtu": 1380, "dns": ["1.1.1.1"], "up": true }
```

---

## 3. 端点

### 3.1 登录 / 登出

登录采用“密码 + TOTP 两步验证”（TOTP 默认开启）。首次登录需先绑定身份
验证器。用于前端渲染的公开端点：

```bash
curl http://localhost:51821/api/auth
# => {"passwordLogin":true,"totpEnabled":true,"totpBound":false}
```

**① 首次登录（未绑定）**：只提交用户名+密码，服务端返回绑定信息（不建会话）：

```bash
curl -s -X POST http://localhost:51821/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"demo"}'
```
```json
{
  "enroll": true,
  "secret": "JBSWY3DPEHPK3PXP",
  "uri": "otpauth://totp/Skyfire:admin?secret=…&issuer=Skyfire&algorithm=SHA1&digits=6&period=30",
  "qr": "data:image/png;base64,…"
}
```

用身份验证器扫码后，带上一次性验证码再次提交，即完成绑定并登录：

```bash
curl -c cookie.txt -X POST http://localhost:51821/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"demo","totp":"123456"}'
# => 200 {"ok":true}   （绑定持久化到 totp.json）
```

**② 已绑定后登录**：不带验证码时会返回需要验证码的提示（仍为 `200`）：

```bash
curl -s -X POST http://localhost:51821/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"demo"}'
# => 200 {"totpRequired":true}
```

带上正确的 6 位验证码后登录成功，响应带 `Set-Cookie: skyfire_session=…`
（HttpOnly，`SameSite=Lax`，TTL 24h）：

```json
{ "ok": true }
```

错误处理：

- 用户名/密码错误：`401 {"error":"invalid credentials"}`
- 验证码错误：`401 {"error":"invalid two-factor code"}`
- 失败过多（同 IP 15 分钟内 10 次）：`429`，带 `Retry-After` 头

> 验证码允许 ±1 个时间步（30s）的时钟偏差。`{"totp":""}` 等同于不提交。

```bash
curl -b cookie.txt -X POST http://localhost:51821/api/logout
# => 200 {"ok": true}
```

### 3.2 健康检查（需认证）

```bash
curl -b cookie.txt http://localhost:51821/api/health
```
```json
{ "status": "ok", "driver": "netstack", "dryRun": false }
```

### 3.3 设置

```bash
# 读取
curl -b cookie.txt http://localhost:51821/api/settings
# => {"publicEndpoint":"vpn.example.com"}

# 更新
curl -b cookie.txt -X PUT http://localhost:51821/api/settings \
  -H 'Content-Type: application/json' \
  -d '{"publicEndpoint":"192.168.1.10"}'
```

### 3.4 接口

```bash
# 列表
curl -b cookie.txt http://localhost:51821/api/interfaces

# 创建（私钥留空自动生成）
curl -b cookie.txt -X POST http://localhost:51821/api/interfaces \
  -H 'Content-Type: application/json' \
  -d '{"name":"wg0","listenPort":51820,"addresses":["10.42.0.1/24"],"dns":["1.1.1.1"],"up":true}'
# => 201 InterfaceView

# 查询单个
curl -b cookie.txt http://localhost:51821/api/interfaces/wg0

# 更新（部分字段）
curl -b cookie.txt -X PUT http://localhost:51821/api/interfaces/wg0 \
  -H 'Content-Type: application/json' \
  -d '{"mtu":1380,"up":false}'

# 启停
curl -b cookie.txt -X POST http://localhost:51821/api/interfaces/wg0/up \
  -H 'Content-Type: application/json' -d '{"up":true}'

# 删除
curl -b cookie.txt -X DELETE http://localhost:51821/api/interfaces/wg0
# => 204
```

约束：`name` 仅小写字母/数字/`-`/`_`，≤15 字符；`listenPort` 不得与其他接口重复；
地址必须是合法 CIDR；`mtu` 范围 576–65535。

### 3.5 接口导出

```bash
# 服务端 wg-quick 配置（text/plain，附件 wg0-server.conf）
curl -b cookie.txt http://localhost:51821/api/interfaces/wg0/config

# 接口私钥（JSON）
curl -b cookie.txt http://localhost:51821/api/interfaces/wg0/private-key
# => {"privateKey":"…base64…"}
```

### 3.6 Peer

```bash
IFACE=wg0

# 添加（自动生成密钥、自动分配地址、带 PSK）
curl -b cookie.txt -X POST http://localhost:51821/api/interfaces/$IFACE/peers \
  -H 'Content-Type: application/json' \
  -d '{
        "name":"laptop",
        "generateKeys":true,
        "withPreshared":true,
        "clientRoutes":["0.0.0.0/0","::/0"],
        "dns":["1.1.1.1"],
        "persistentKeepalive":25,
        "enabled":true
      }'
# => 201 PeerView（含 clientToken）

# 更新（key 为当前公钥的 url-safe base64）
PUBKEY='<base64 standard>'
KEY="$(printf '%s' "$PUBKEY" | tr '+/' '-_' | tr -d '=')"
curl -b cookie.txt -X PUT http://localhost:51821/api/interfaces/$IFACE/peers/$KEY \
  -H 'Content-Type: application/json' \
  -d '{"name":"laptop","address":"10.42.0.2","clientRoutes":["10.0.0.0/8"],
       "persistentKeepalive":25,"enabled":true,"withPreshared":false}'

# 删除
curl -b cookie.txt -X DELETE http://localhost:51821/api/interfaces/$IFACE/peers/$KEY
# => 204

# 客户端配置（text/plain，附件 wg0.conf）
curl -b cookie.txt http://localhost:51821/api/interfaces/$IFACE/peers/$KEY/config

# 客户端配置二维码（image/png，无缓存）
curl -b cookie.txt -o peer.png \
  http://localhost:51821/api/interfaces/$IFACE/peers/$KEY/config.png
```

> `{"generateKeys":true}` 会**轮换**该 Peer 的密钥对；旧的公钥路径随即失效。

### 3.7 令牌端点（免登录）

用于桌面客户端一键连接。`token` 即 Peer 的 `clientToken`（在 Web UI 的
Peer 详情「Desktop client」页签可见），只暴露该 Peer 的配置。

```bash
TOKEN='<clientToken>'

# 客户端配置
curl http://localhost:51821/api/p/$TOKEN/wg.conf

# 二维码
curl -o peer.png http://localhost:51821/api/p/$TOKEN/wg.png
```

token 无效：`404 {"error":"not found: peer"}`。

---

## 4. 完整示例（脚本化建接口 + Peer）

```bash
HOST=http://localhost:51821
AUTH='Authorization: Bearer demo'   # 或改用 -c cookie.txt

curl -s -H "$AUTH" -X POST "$HOST/api/settings" \
  -H 'Content-Type: application/json' -d '{"publicEndpoint":"vpn.example.com"}' >/dev/null

curl -s -H "$AUTH" -X POST "$HOST/api/interfaces" \
  -H 'Content-Type: application/json' \
  -d '{"name":"wg0","listenPort":51820,"addresses":["10.42.0.1/24"],"dns":["1.1.1.1"],"up":true}' >/dev/null

PEER=$(curl -s -H "$AUTH" -X POST "$HOST/api/interfaces/wg0/peers" \
  -H 'Content-Type: application/json' \
  -d '{"name":"laptop","generateKeys":true,"clientRoutes":["0.0.0.0/0","::/0"],"dns":["1.1.1.1"],"persistentKeepalive":25,"enabled":true}')

TOKEN=$(printf '%s' "$PEER" | sed -n 's/.*"clientToken":"\([^"]*\)".*/\1/p')
echo "连接字符串：$HOST/api/p/$TOKEN/wg.conf"
```

---

## 5. 相关文档

- [服务端 skyfired](server.md)
- [客户端 skyfire-client](client.md)
- [常见问题 / 故障排查](faq.md)
