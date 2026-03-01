# 文件下载安全认证使用文档

本文档介绍 go-fastdfs 的两种文件下载认证方案，适用于不同的使用场景。

---

## 目录

- [方案概览](#方案概览)
- [方案 B: auth_url 缓存认证](#方案-b-auth_url-缓存认证)
- [方案 C: HMAC 签名认证](#方案-c-hmac-签名认证)
- [混合方案配置](#混合方案配置)
- [常见问题](#常见问题)

---

## 方案概览

| 方案 | 适用场景 | 性能 | 安全性 | 客户端改动 |
|------|----------|------|--------|------------|
| **B: auth_url 缓存** | APP 端直接下载 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 无需改动 |
| **C: HMAC 签名** | 服务端间调用 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 需要获取签名 |

### 推荐架构

```
┌─────────────┐                    ┌─────────────┐
│  Flutter    │                    │  API 服务端  │
│    APP      │                    │             │
└──────┬──────┘                    └──────┬──────┘
       │                                  │
       │ 方案 B: auth_url + 缓存          │ 方案 C: HMAC 签名
       │ Header: Auth-Token               │ ?token=xxx&ts=xxx
       │                                  │
       ▼                                  ▼
┌─────────────────────────────────────────────────────┐
│                    go-fastdfs                        │
└─────────────────────────────────────────────────────┘
```

---

## 方案 B: auth_url 缓存认证

### 适用场景

- Flutter/移动端 APP 直接下载文件
- 已有用户认证系统（BFF 服务）
- 需要实时用户权限验证

### 工作原理

```
APP 请求下载
    │
    ▼
检查缓存 ──命中──> 直接返回结果 (<1ms)
    │
   未命中
    │
    ▼
调用 auth_url 验证 (10-50ms)
    │
    ▼
缓存验证结果 (TTL = download_token_expire)
    │
    ▼
返回文件
```

### 配置

```json
{
  "enable_download_auth": true,
  "auth_url": "http://your-bff-server/api/auth/verify",
  "download_token_expire": 300
}
```

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `enable_download_auth` | 是否启用下载认证 | `false` |
| `auth_url` | 认证服务 URL | `""` |
| `download_token_expire` | 认证结果缓存时间(秒) | `600` |

### 客户端使用

**请求示例：**

```http
GET /group1/default/20260301/report.pdf HTTP/1.1
Host: fastdfs.example.com
Auth-Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Flutter 示例：**

```dart
import 'package:dio/dio.dart';

Future<void> downloadFile(String filePath, String authToken) async {
  final dio = Dio();

  await dio.download(
    'http://fastdfs.example.com$filePath',
    '/path/to/local/file.pdf',
    options: Options(
      headers: {
        'Auth-Token': authToken,
      },
    ),
  );
}
```

### auth_url 服务端要求

您的认证服务需要：

1. 接收 POST 请求
2. 从请求头或参数中获取 `Auth-Token`
3. 验证用户身份和权限
4. 返回验证结果

**请求格式：**

```http
POST /api/auth/verify HTTP/1.1
Content-Type: application/x-www-form-urlencoded
Auth-Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**响应格式（成功）：**

```
ok
```

或

```json
{"data": "ok"}
```

**响应格式（失败）：**

```json
{"data": "fail", "message": "权限不足"}
```

### Node.js auth_url 服务示例

```javascript
const express = require('express');
const jwt = require('jsonwebtoken');

const app = express();

app.use(express.urlencoded({ extended: true }));
app.use(express.json());

app.post('/api/auth/verify', async (req, res) => {
  const authToken = req.headers['auth-token'];

  if (!authToken) {
    return res.send('fail');
  }

  try {
    // 验证 JWT
    const decoded = jwt.verify(authToken, process.env.JWT_SECRET);

    // 检查用户权限（可选）
    const userId = decoded.userId;
    const hasPermission = await checkUserPermission(userId);

    if (hasPermission) {
      return res.send('ok');
    }
  } catch (err) {
    console.error('Auth error:', err);
  }

  return res.send('fail');
});

app.listen(3000);
```

---

## 方案 C: HMAC 签名认证

### 适用场景

- 服务端间文件下载
- 后台任务批量处理
- 需要最高性能的场景

### 工作原理

```
服务端生成签名 URL
    │
    │ token = HMAC-SHA256(secret_key, path|timestamp)
    │
    ▼
返回签名 URL 给调用方
    │
    ▼
调用方使用签名 URL 下载
    │
    ▼
go-fastdfs 本地验证签名 (<1ms)
    │
    ▼
返回文件
```

### 配置

```json
{
  "enable_hmac_auth": true,
  "download_secret_key": "your-secret-key-at-least-32-characters-long",
  "download_token_expire": 300
}
```

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `enable_hmac_auth` | 是否启用 HMAC 认证 | `false` |
| `download_secret_key` | HMAC 签名密钥 | `""` |
| `download_token_expire` | 签名有效期(秒) | `600` |

### API 接口

**获取签名 URL**

```http
GET /{group}/sign_download?path=/group1/default/20260301/report.pdf
```

**响应：**

```json
{
  "code": 0,
  "data": {
    "url": "/group1/default/20260301/report.pdf?token=a1b2c3d4...&ts=1709251200",
    "token": "a1b2c3d4e5f6...",
    "timestamp": "1709251200",
    "expire": "300"
  }
}
```

### 客户端使用

**Go 示例：**

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type SignResponse struct {
    Code int `json:"code"`
    Data struct {
        URL       string `json:"url"`
        Token     string `json:"token"`
        Timestamp string `json:"timestamp"`
        Expire    string `json:"expire"`
    } `json:"data"`
}

func main() {
    fastdfsURL := "http://fastdfs.example.com"
    filePath := "/group1/default/20260301/report.pdf"

    // 1. 获取签名 URL
    signURL := fmt.Sprintf("%s/group1/sign_download?path=%s", fastdfsURL, filePath)
    resp, _ := http.Get(signURL)
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    var signResp SignResponse
    json.Unmarshal(body, &signResp)

    // 2. 使用签名 URL 下载
    downloadURL := fastdfsURL + signResp.Data.URL
    fmt.Println("Download URL:", downloadURL)

    // 有效期内可多次使用
    resp2, _ := http.Get(downloadURL)
    defer resp2.Body.Close()

    // 处理文件内容...
}
```

**curl 示例：**

```bash
# 1. 获取签名 URL
SIGN_URL=$(curl -s "http://fastdfs.example.com/group1/sign_download?path=/group1/default/20260301/report.pdf" | jq -r '.data.url')

# 2. 使用签名 URL 下载
curl -O "http://fastdfs.example.com$SIGN_URL"
```

### 自行生成签名（无需调用 API）

如果您的服务端也需要生成签名，可以使用以下算法：

**签名算法：**

```
message = file_path + "|" + timestamp
token = HMAC-SHA256(secret_key, message)
```

**Go 实现：**

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strconv"
    "time"
)

func generateSignedURL(filePath, secretKey string, expireSeconds int) string {
    timestamp := strconv.FormatInt(time.Now().Unix(), 10)
    message := filePath + "|" + timestamp

    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(message))
    token := hex.EncodeToString(h.Sum(nil))

    return fmt.Sprintf("%s?token=%s&ts=%s", filePath, token, timestamp)
}

// 使用
signedURL := generateSignedURL("/group1/default/file.pdf", "your-secret-key", 300)
```

**Node.js 实现：**

```javascript
const crypto = require('crypto');

function generateSignedURL(filePath, secretKey) {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const message = `${filePath}|${timestamp}`;
  const token = crypto
    .createHmac('sha256', secretKey)
    .update(message)
    .digest('hex');

  return `${filePath}?token=${token}&ts=${timestamp}`;
}

// 使用
const signedURL = generateSignedURL('/group1/default/file.pdf', 'your-secret-key');
```

---

## 混合方案配置

### 完整配置示例

```json
{
  "addr": ":8080",
  "group": "group1",

  "enable_download_auth": true,
  "auth_url": "http://your-bff-server/api/auth/verify",
  "download_token_expire": 300,

  "enable_hmac_auth": true,
  "download_secret_key": "your-secret-key-at-least-32-characters-long"
}
```

### 验证优先级

当两种方案同时启用时，验证顺序为：

1. **优先检查 HMAC 签名** - 如果请求包含 `token` 和 `ts` 参数
2. **再检查 auth_url** - 如果请求包含 `Auth-Token` 头

### 最佳实践

| 场景 | 推荐配置 |
|------|----------|
| 仅 APP 使用 | 只启用 `enable_download_auth` |
| 仅服务端使用 | 只启用 `enable_hmac_auth` |
| 混合使用 | 两者都启用 |

---

## 常见问题

### Q1: auth_url 缓存会导致权限变更不及时吗？

**A:** 缓存 TTL 由 `download_token_expire` 控制（默认 600 秒）。如果需要更实时的权限控制，可以：
1. 减少 `download_token_expire` 值
2. 在用户权限变更时，调用 `/reload` API 清除缓存

### Q2: HMAC 签名的 secret_key 应该如何管理？

**A:**
1. 密钥长度建议至少 32 字符
2. 不要在客户端（APP/前端）存储密钥
3. 定期更换密钥
4. 使用环境变量或密钥管理服务存储

### Q3: 签名 URL 可以分享给他人吗？

**A:** 签名 URL 在有效期内可以被任何人使用。如果需要更严格的控制：
1. 缩短有效期 `download_token_expire`
2. 实现一次性令牌（需要额外开发）

### Q4: 如何测试认证是否正常工作？

**测试 auth_url：**
```bash
# 应返回 401
curl -I http://fastdfs:8080/group1/default/file.pdf

# 携带有效 token 应返回 200
curl -I -H "Auth-Token: valid-token" http://fastdfs:8080/group1/default/file.pdf
```

**测试 HMAC 签名：**
```bash
# 获取签名 URL
SIGNED_URL=$(curl -s "http://fastdfs:8080/group1/sign_download?path=/group1/default/file.pdf" | jq -r '.data.url')

# 使用签名 URL
curl -I "http://fastdfs:8080$SIGNED_URL"
```

### Q5: 性能对比如何？

| 场景 | 无缓存 | 有缓存 | HMAC 签名 |
|------|--------|--------|-----------|
| 单次验证延迟 | 10-50ms | <1ms | <1ms |
| 100 次相同请求 | 1-5s | 10-50ms + 99×<1ms | 100×<1ms |

---

## 更新日志

- **2026-03-01**: 添加混合认证方案 (auth_url 缓存 + HMAC 签名)
