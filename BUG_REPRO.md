# 资源可见性诊断

## Bug 是什么

未登录访问公开资源列表时，服务可能返回列表结果而不是拒绝访问。不同入口对身份缺失的处理不一致，造成可见性边界失真。

## 如何触发

在没有有效身份的情况下请求资源列表，并经过列表服务进入项目可见性查询。若请求未在入口被拒绝，查询会继续执行并返回可见性结果。

## 根因

文件：internal/dashboardservice/dashboardservice.go；符号：Service.ListForUser；失效机制：状态一致性与跨层数据流失配。列表服务删除了对空身份的拒绝判断，查询层仍把空身份当作普通筛选条件处理，业务层和查询层不再共享同一个访问边界，导致匿名请求绕过应有的授权语义。

## 运行指令

```bash
go test ./internal/dashboardservice -v -run '^TestAnonymousProjectIndexRequiresIdentity$' -count=1
```

## 错误信息

匿名请求没有得到拒绝错误，说明资源列表入口没有维持身份前置条件。

## 错误堆栈

```text
--- FAIL: TestAnonymousProjectIndexRequiresIdentity (0.00s)
    visibility_target_test.go:30: anonymous project index error=<nil>, want forbidden
FAIL
```
