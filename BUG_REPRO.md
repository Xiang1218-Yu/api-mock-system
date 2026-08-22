# 版本回退结构诊断

## Bug 是什么

回退到历史版本后，复杂请求结构会丢失，文档和模拟响应因此不能按照原有结构理解数据。

## 如何触发

保存包含命名映射的接口快照后执行回退。快照恢复过程读取结构字段时只接受一种映射表示，另一种已序列化的表示无法赋回接口。

## 根因

文件：internal/apiservice/api_versions.go；符号：restoreAPIFromSnapshot；失效机制：跨层数据流中的运行时类型恢复不完整。版本快照经过持久化后以命名映射表示复杂结构，恢复逻辑只匹配通用映射，导致请求和响应结构在回退时被静默丢弃，并传播到文档构建与模拟解析链路。

## 运行指令

```bash
go test ./internal/apiservice -v -run '^TestRollbackRestoresNamedSchemaMaps$' -count=1
```

## 错误信息

回退后的请求和响应结构为空，说明快照中的命名映射没有恢复到接口定义。

## 错误堆栈

```text
--- FAIL: TestRollbackRestoresNamedSchemaMaps (0.00s)
    rollback_target_test.go:17: rollback dropped named schema maps: request=map[] response=map[]
FAIL
```
