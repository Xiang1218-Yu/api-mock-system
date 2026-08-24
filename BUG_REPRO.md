# Bug 024 复现说明

## Bug 是什么
调用耗时落在 10ms 或 50ms 等分桶边界时，统计结果会被放进错误的区间，返回的桶顺序也可能与看板使用的约定不一致，导致耗时分布图展示错误。

## 如何触发
准备耗时分别落在 10ms、50ms 和更高区间的调用记录，再请求看板统计。修复前，仓储层的边界判断和服务层的桶重组使用了不同约定，处理器最终返回的标签和顺序因此偏离图表预期；修复后，边界归类和输出顺序保持一致。

## 根因
`internal/calllogrepo/calllogrepo.go` 对耗时上限的比较方式与桶定义不一致，`internal/dashboardservice/dashboardservice.go` 又按另一套顺序聚合结果，`internal/dashboardhandler/dashboardhandler.go` 直接输出了未统一的桶序列。三层之间的统计状态契约不一致，造成边界调用被错误归类。

## 运行指令
```bash
go test -v -count=1 ./internal/calllogrepo -run '^TestBug024DurationBucketsRespectBoundaries$'
```

## 错误信息
修复前，恰好 10ms 的调用被放入 `0-10ms`，而测试要求它进入 `10-50ms` 区间。

## 错误堆栈
```text
=== RUN   TestBug024DurationBucketsRespectBoundaries
    bug024_duration_buckets_test.go:7: bucketLabel(10)="0-10ms", want 10-50ms
--- FAIL: TestBug024DurationBucketsRespectBoundaries (0.00s)
FAIL
FAIL	api-mock-system/internal/calllogrepo	0.636s
FAIL
```

## 修复后结果
修复后，同一条验证指令能够通过，耗时边界按照统一规则归类，看板桶序列也保持稳定。
