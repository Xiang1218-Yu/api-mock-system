# 成员移除保护诊断

## Bug 是什么

成员移除请求携带带空白的所有者标识时，所有者保护可能未生效，删除路径会继续执行。

## 如何触发

由具备成员管理权限的用户提交移除请求，并在目标成员标识两侧加入空白。若目标在规范化前直接参与所有者比较，保护判断会失配。

## 根因

文件：internal/projectservice/projectservice.go；符号：Service.RemoveMember；失效机制：跨层数据流中的身份标识未统一规范化。请求边界、所有者比较和持久化删除使用了不同的标识语义，带空白的目标值无法命中所有者保护，随后沿授权和删除链路继续传播。

## 运行指令

```bash
go test ./internal/projectservice -v -run '^TestRemoveMemberDoesNotBypassOwnerProtectionWithWhitespace$' -count=1
```

## 错误信息

带空白的所有者标识没有被识别为受保护目标，操作返回了成功结果。

## 错误堆栈

```text
--- FAIL: TestRemoveMemberDoesNotBypassOwnerProtectionWithWhitespace (0.00s)
    member_removal_target_test.go:33: owner removal with whitespace error=<nil>, want forbidden
FAIL
```
