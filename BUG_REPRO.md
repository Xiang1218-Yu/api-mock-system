# Bug 复现说明

## Bug 是什么

调试调用在下游模拟查询失败后会向调用方返回错误，但同一次失败不会进入历史记录。于是调用方已经观察到失败，后续排查却无法确定失败调用是否发生过。

## 如何触发

创建一条可访问的已发布接口，在模拟查询阶段让请求上下文结束，再从调试入口发起调用。调用会走失败分支；该分支应保存失败记录，但含缺陷版本把已结束的上下文继续交给日志仓储。

## 运行指令

```bash
go test ./internal/debughandler -v -run '^TestFailedDebugRequestRetainsHistoryRecord$' -count=1
```

## 错误信息

含缺陷版本的日志仓储在保存失败调用时收到已取消的上下文，因此拒绝写入历史记录。

## 错误堆栈

```text
--- FAIL: TestFailedDebugRequestRetainsHistoryRecord
    v5_contract_test.go:108: failed debug invocation was not retained: context canceled
FAIL
FAIL    api-mock-system/internal/debughandler
```

## 根因

文件：internal/debughandler/debughandler.go；文件：internal/debugservice/debugservice.go；文件：internal/debugrepo/debugrepo.go；文件：internal/models/models.go。符号：`Service.Debug`。失败调用到达模拟服务后，请求 context 已结束，`Service.Debug` 的失败分支仍把该 context 传入日志保存入口。日志仓储以该 context 执行写入并拒绝已取消的操作，因而错误响应已经返回，失败历史却不存在。

失效机制属于 context 生命周期失配：成功记录使用独立后台保存路径，而失败记录继承请求生命周期。应让失败记录拥有独立且有界的保存生命周期，同时保留仓储对真正取消操作的错误传播语义。本诊断未修改生产代码。
