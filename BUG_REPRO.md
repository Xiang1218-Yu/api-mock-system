# Bug 复现说明

## Bug 是什么

清理一个接口的固定响应时，缓存失效范围可能覆盖到标识相邻的另一接口，导致不相关请求丢失缓存；替换规则后也可能保留与规则状态不一致的响应结果。

## 如何触发

缓存两个具有前缀关系的接口响应，随后只对较短标识执行规则失效。正确行为应仅删除该接口自己的缓存记录。

## 运行指令

```bash
go test ./internal/mockservice -v -run '^TestInvalidateKeepsAdjacentAPIKeys$' -count=1
```

## 错误信息

含缺陷版本会发现相邻接口的缓存也被清理。

## 错误堆栈

```text
--- FAIL: TestInvalidateKeepsAdjacentAPIKeys
    v5_contract_test.go:22: cache invalidation removed an adjacent API entry
FAIL
FAIL    api-mock-system/internal/mockservice
```

## 根因

文件：internal/apiservice/apiservice.go；文件：internal/mockdatarepo/mockdatarepo.go；文件：internal/mockhandler/mockhandler.go；文件：internal/mockservice/mockservice.go。`Service.Invalidate` 依据接口标识拼接缓存前缀，却没有保留接口标识与后续缓存字段之间的分隔边界。当前缀关系存在时，短标识可匹配长标识的缓存键，导致规则存储的变更范围与缓存失效范围不再一致。

修正边界应让规则键的规范化、HTTP 入口的键传递和缓存失效都使用完整、可区分的接口键。该诊断未修改生产代码。
