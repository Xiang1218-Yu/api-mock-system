# Bug 复现说明

## Bug 是什么
同一项目邀请不同成员时，后续邀请可能被错误拒绝，成员关系表和接口返回结果也会偏离实际邀请情况；重复邀请和角色错误仍应维持原有规则。

## 如何触发
以项目管理员身份先邀请一名成员，再邀请同一项目中的另一名成员，随后检查成员列表、重复邀请和非法角色请求的结果。

## 根因
成员关系的唯一性约束错误地只覆盖项目字段，导致同一项目的第二条成员记录触发数据库冲突；仓储和服务层又把该冲突传播成了错误的禁止响应。

## 运行指令
```bash
go test -v -count=1 ./internal/verification -run '^TestProjectCanInviteMultipleMembers$'
```

## 错误信息
修复前首次成员邀请就可能因数据库唯一约束冲突返回 `forbidden`，后续成员无法正常加入项目。

## 错误堆栈
```text
UNIQUE constraint failed: project_members.project_id；goroutine 37 [running]: runtime/debug.Stack -> stack trace；first member invite failed: forbidden
UNIQUE constraint failed: project_members.project_id
goroutine 37 [running]: runtime/debug.Stack -> stack trace
first member invite failed: forbidden
=== RUN   TestProjectCanInviteMultipleMembers
    bug019_test.go:39: first member invite failed: forbidden
--- FAIL: TestProjectCanInviteMultipleMembers
FAIL
FAIL    api-mock-system/internal/verification
```
