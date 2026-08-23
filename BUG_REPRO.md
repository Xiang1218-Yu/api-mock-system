# Bug 复现说明

## Bug 是什么
批量 JSON 请求中位于数组后部的违规项可能被忽略，服务只返回前面的错误，甚至接受本应拒绝的整批数据。

## 如何触发
提交包含多个数组元素的 JSON 请求，让前部和后部元素分别违反不同校验规则，再检查服务是否遍历全部元素并汇总完整错误。

## 根因
请求体数字归一化、数组递归校验和服务层错误返回都存在只保留第一项的截断逻辑，后部元素在跨层传递过程中被丢弃，最终错误集合也被截断。

## 运行指令
```bash
go test -v -count=1 ./internal/mockservice -run '^TestBatchRequestValidationChecksEveryArrayElement$'
```

## 错误信息
修复前错误信息只保留数组前部的校验结果，未能报告其他无效元素。

## 错误堆栈
```text
=== RUN   TestBatchRequestValidationChecksEveryArrayElement
    bug015_verification_test.go:27: validation error omitted an invalid array element: mock request does not match request schema: schema validation failed: $[0] (minLength): must contain at least 3 characters
--- FAIL: TestBatchRequestValidationChecksEveryArrayElement
FAIL
FAIL    api-mock-system/internal/mockservice
```
