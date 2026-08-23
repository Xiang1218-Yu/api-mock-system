# Bug 016 复现说明

## Bug 是什么

数组响应的结构定义不完整时，mock 服务在生成响应的过程中可能发生运行时崩溃，调用方收不到正常的 HTTP 结果。合法的数组响应也需要保持原有的数组形状并正常返回。

## 如何触发

准备一个已发布接口，将响应结构声明为数组但不提供 `items` 元素定义，然后通过 mock 服务请求该接口。修复前，数组生成路径会对缺失的元素结构做不安全处理，最终触发崩溃；修复后，生成器能够对缺失或异常结构使用安全的默认元素，服务不再因该输入崩溃。

## 根因

`internal/mockengine/engine.go` 的数组生成逻辑没有完整区分缺失、单对象和数组形式的 `items` 定义；`internal/mockservice/mockservice.go` 还会提前解包数组元素并进行不安全的类型断言，导致不完整的 JSON 结构进入运行时崩溃路径。服务层和处理器层缺少将生成异常转换为受控服务错误的边界，因此异常会直接影响 mock 请求。

## 运行指令

```bash
go test -v -count=1 ./internal/verification -run '^TestArrayResponseSchemaWithoutItemsDoesNotPanic$'
```

## 错误信息

修复前测试捕获到数组响应生成过程中的运行时崩溃，目标行为是“不发生 panic 并生成一个数组元素”。

## 错误堆栈

```text
=== RUN   TestArrayResponseSchemaWithoutItemsDoesNotPanic
    bug016_test.go:14: array response generation panicked: interface conversion: interface {} is nil, not map[string]interface {}
--- FAIL: TestArrayResponseSchemaWithoutItemsDoesNotPanic (0.00s)
FAIL
FAIL	api-mock-system/internal/verification	0.476s
```

## 修复后结果

修复后同一条验证指令连续执行五轮均通过，数组响应能够生成结果，异常结构不会再导致该运行时崩溃。
