# 接口发布状态诊断

## Bug 是什么

接口从异常版本状态发布后会显示为可调用，但版本仍是无效值。历史状态与实时调用状态因此不满足同一发布约束。

## 如何触发

创建一个版本值不为正的接口并执行发布。发布流程会写入版本记录并将接口标记为已发布，但没有先将版本修复到有效范围。

## 根因

文件：internal/apiservice/api_versions.go；符号：Service.Publish；失效机制：错误传播与状态一致性失配。发布路径直接以当前版本计算下一版本并提交可调用状态，缺少对非正版本的下限修复；仓储快照、接口状态和模拟调用随后看到彼此矛盾的发布结果。

## 运行指令

```bash
go test ./internal/apiservice -v -run '^TestPublishRepairsNonPositiveVersionBeforeMakingRouteLive$' -count=1
```

## 错误信息

发布结果已经标记为可调用，但版本值仍为零，违反发布状态必须同时有效的约束。

## 错误堆栈

```text
--- FAIL: TestPublishRepairsNonPositiveVersionBeforeMakingRouteLive (0.00s)
    publish_target_test.go:41: published version=0 status="published", want version 1 and published
FAIL
```
