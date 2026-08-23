# Bug 复现说明

## Bug 是什么
同一个已发布接口收到不同查询参数或不同 JSON 请求体时，mock 响应可能复用上一次请求的结果；固定响应覆盖和普通响应生成也会互相影响。

## 如何触发
连续请求同一个 mock 接口，先后改变查询参数和请求体内容，再检查每次响应是否分别对应自己的请求变体，并确认固定响应仍保持原有语义。

## 根因
请求处理器传递给服务层的输入不完整，缓存键和确定性生成种子没有同时区分查询参数与请求体，导致不同请求共享同一个响应身份。

## 运行指令
```bash
go test -v -count=1 ./internal/mockservice -run '^TestRequestVariantsKeepQueryAndBodySeparate$'
```

## 错误信息
修复前不同请求变体共享同一个 mock 缓存身份，测试以退出码 1 失败。

## 错误堆栈
```text
=== RUN   TestRequestVariantsKeepQueryAndBodySeparate
    bug014_verification_test.go:15: request variants share one mock cache identity
--- FAIL: TestRequestVariantsKeepQueryAndBodySeparate
FAIL
FAIL    api-mock-system/internal/mockservice
```
