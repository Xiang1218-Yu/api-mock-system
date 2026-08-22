# Bug 复现说明

## Bug 是什么

缓存条目在生命周期刚好结束时仍可能被视为有效，前缀清理还会把它统计为一次有效删除，导致响应、删除数量与存活数量不一致。

## 如何触发

固定缓存时钟后写入一条有生命周期的记录，并将时间推进到其失效瞬间；随后让清理逻辑在并发路径中处理该前缀。

## 运行指令

```bash
go test ./internal/cache -race -v -run '^TestExpiryBoundaryIsNotCountedByConcurrentCleanup$' -count=20
```

## 错误信息

含缺陷版本会把刚好到期的条目记为已删除的有效缓存项。

## 错误堆栈

```text
--- FAIL: TestExpiryBoundaryIsNotCountedByConcurrentCleanup
    v5_contract_test.go:21: expired entry was counted as deleted: 1
FAIL
FAIL    api-mock-system/internal/cache
```

## 根因

文件：internal/cache/cache.go；文件：internal/app/app.go；文件：internal/mockhandler/mockhandler.go；文件：internal/mockservice/mockservice.go。`Cache.Get`、`Cache.DeleteByPrefix`、`Cache.Purge` 和 `Cache.Len` 对失效时刻采用了不同的时间边界判断：刚好等于到期时间的条目没有被统一视为过期。于是读取路径可能继续使用旧值，前缀清理又会将它计入有效删除，后台清理与数量展示随之产生不同结果。

修正边界应将缓存核心的存活判断统一为同一时间语义，并确保应用的后台清理、模拟响应读取和请求处理链路都遵循该语义。该诊断未修改生产代码。
