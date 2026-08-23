# Bug 复现说明

## Bug 是什么
并发注册使用同一邮箱时，系统可能同时创建多个账户；邮箱只改变大小写时，也可能被当成不同身份处理，导致重复注册没有返回冲突。

## 如何触发
让多个请求同时提交相同邮箱的注册请求，并混用不同大小写的邮箱字符串，观察 HTTP 注册结果和最终用户记录数量。

## 根因
注册链路在并发写入前进行的检查与创建没有形成一致的唯一性约束，邮箱规范化也没有在持久化和查询链路中统一生效。数据库冲突、服务层错误和处理器返回状态因此没有保持一致。

## 运行指令
```bash
go test -v -race -count=20 ./internal/verification -run '^TestRegisterConcurrentDuplicateEmailReturnsConflict$'
```

## 错误信息
修复前重复注册请求返回 `201 Created`，而调用方需要得到 `409 Conflict`；该结果在 20 次运行中稳定出现。

## 错误堆栈
```text
goroutine 17 [running]: runtime/debug.Stack -> stack trace；duplicate registration status = 201, want 409
=== RUN   TestRegisterConcurrentDuplicateEmailReturnsConflict
    bug013_test.go:139: duplicate registration status = 201, want 409
--- FAIL: TestRegisterConcurrentDuplicateEmailReturnsConflict
FAIL
FAIL    api-mock-system/internal/verification
```
