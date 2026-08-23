# Bug 020 复现说明

## Bug 是什么

文档导出流程没有按请求格式返回内容：选择 YAML 时实际得到 JSON；同时，OpenAPI 构建过程可能遗漏已发布接口，导致导出的文档不能完整描述项目能力。

## 如何触发

创建包含多个已发布接口的项目，分别请求 JSON 和 YAML 文档导出。修复前，YAML 导出路径仍调用 JSON 序列化器，且 OpenAPI 操作映射使用了与构建数据不一致的 HTTP 方法表示，部分操作因此没有写入文档；修复后，格式选择、文件名、内容类型和已发布操作集合保持一致。

## 根因

`internal/docservice/docservice.go` 的 YAML 导出仍返回 JSON 序列化结果，`internal/dochandler/dochandler.go` 也沿用了 JSON 的响应媒体类型和文件名。与此同时，`internal/openapi/openapi.go` 的操作分派只识别大写方法，而构建流程传入的是小写方法，未命中的操作被静默丢弃，造成已发布接口缺失。

## 运行指令

```bash
go test -v -count=1 ./internal/verification -run '^TestDocumentExportPreservesYAMLAndPublishedOperations$'
```

## 错误信息

修复前，验证用例发现 YAML 导出内容仍是 JSON，因而无法继续确认导出的接口集合是否符合请求格式。

## 错误堆栈

```text
=== RUN   TestDocumentExportPreservesYAMLAndPublishedOperations
    bug020_test.go:64: YAML export is not YAML: {
          "openapi": "3.0.3",
          "info": {
            "title": "documentation project",
            "version": "1.0.0"
          },
          "paths": {
            "/users": {}
          }
        }
--- FAIL: TestDocumentExportPreservesYAMLAndPublishedOperations (0.00s)
FAIL
FAIL	api-mock-system/internal/verification	0.760s
```

## 修复后结果

修复后同一条验证指令连续执行五轮均通过，YAML 导出返回 YAML 内容，OpenAPI 文档也保留了已发布的接口操作。
