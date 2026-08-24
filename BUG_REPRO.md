# Bug 026 复现说明

## Bug 是什么
创建并发布 POST 接口后，客户端按照接口登记的地址请求却得到未找到，而其他请求方式的接口可以正常匹配。接口方法在创建、保存和运行时查找之间没有保持一致。

## 如何触发
创建一个 POST 接口并发布，再使用 POST 请求访问登记路径。修复前，接口写入阶段保存的方法格式与运行时路由匹配使用的格式不同，Mock 入口找不到已经发布的接口；修复后，创建和匹配使用同一套规范化方法。

## 根因
`internal/apiservice/api_helpers.go` 产生接口方法标识，`internal/apiservice/api_mock_access.go` 负责运行时查找，`internal/mockhandler/mockhandler.go` 将真实 HTTP 方法传入匹配链路。三层对方法大小写和规范形式的理解不同，造成 POST 请求在跨层数据流中变成未匹配的方法。

## 运行指令
```bash
go test -v -count=1 ./internal/apiservice -run '^TestBug026MethodRouteUsesCanonicalHTTPMethod$'
```

## 错误信息
修复前，方法规范化函数返回了小写的 `post`，而路由匹配要求规范化后的方法为 `POST`。

## 错误堆栈
```text
=== RUN   TestBug026MethodRouteUsesCanonicalHTTPMethod
    bug026_method_route_test.go:7: normalizeMethod(post)="post", want POST
--- FAIL: TestBug026MethodRouteUsesCanonicalHTTPMethod (0.00s)
FAIL
FAIL	api-mock-system/internal/apiservice	0.450s
FAIL
```

## 修复后结果
修复后，同一条验证指令能够通过，已发布的 POST 接口可以按登记的方法和路径被 Mock 入口匹配。
