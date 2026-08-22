# Bug 复现说明

## Bug 是什么

不同用户或不同项目在并发访问时会落入同一个限流状态，导致彼此消耗访问额度，其中一方会收到本不应出现的限流响应。

## 如何触发

为两个具有相同尾部标识、但属于不同作用域的并发请求同时进入限流入口。它们应各自获得独立的初始额度；含缺陷版本会让其中一个请求被拒绝。

## 运行指令

```bash
go test ./internal/middleware -race -v -run '^TestRateLimitSeparatesUserAndProjectScopes$' -count=20
```

## 错误信息

含缺陷版本中，目标检查会持续观察到独立作用域的请求被限流。

## 错误堆栈

```text
--- FAIL: TestRateLimitSeparatesUserAndProjectScopes
    v5_contract_test.go:40: independent scope was rate limited: status=429
FAIL
FAIL    api-mock-system/internal/middleware
```

## 根因

文件：internal/router/router.go；文件：internal/middleware/middleware.go；文件：internal/config/config.go；文件：internal/app/app.go。路由层在不同入口生成带作用域的用户或项目键，`middleware.RateLimit` 又在选择内存令牌桶前剥离了这部分作用域。两个键因此被压缩为同一个桶标识，后到达的并发请求会消耗前一个请求所属主体的额度。配置与应用初始化负责传入限流参数，虽然它们保持了参数的有效范围，却无法恢复已经丢失的身份边界。

修正应让带作用域的键完整进入桶索引，并验证路由入口、限流中间件、配置装配和应用初始化对同一主体边界使用一致语义。该诊断未修改生产代码。
