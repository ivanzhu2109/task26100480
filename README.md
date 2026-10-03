# plansched — 工程进度与总时差分析库

Go 库，由上层直接调用，计划与计算结果保存在进程内存中。时间统一为相对项目起点的整数工作小时，结果精确到整数。

## 模型

- `Plan`：活动列表 + 依赖列表 + 承诺完工时点 `Deadline`（T）。
- `Activity`：`ID` + 非负工期（允许零工期里程碑）；完成时点 = 开始时点 + 工期。
- `Dependency`：从前项指向后项，带整数时距 `Lag`（可为负），同一对活动允许多条依赖，全部同时满足。四种关系：
  - `FinishToStart`：`Sj >= Si + Di + L`
  - `StartToStart`：`Sj >= Si + L`
  - `FinishToFinish`：`Sj >= Si + Di - Dj + L`
  - `StartToFinish`：`Sj >= Si + L - Dj`

## 调用

```go
res, err := plansched.Analyze(plan)          // 整份计划一次性分析

s, err := plansched.NewSchedule(plan)        // 持有计划，持续修改重分析
err = s.SetDuration("C", 6)                  // 改工期并重分析
err = s.SetLag("d2", -2)                     // 改时距并重分析
res = s.Result()                             // 取最新结果
```

- 每次修改都基于完整计划从头计算，结果与重新提交完整计划一致；修改导致依赖矛盾时返回 `ErrInfeasible` 并回滚。
- `Result.Activities[id]`：最早开始/完成、最晚开始/完成、总时差（最晚开始 − 最早开始，负值为赶工缺口）。
- `Result.ProjectEarliestFinish`：项目最早完工时点；`Result.NoFloat`：总时差 ≤ 0 的活动。
- 最早计划要求开始 ≥ 0；最晚计划要求完成 ≤ T，允许开始为负，不截断。
- 依赖闭环可行则正常排程；依赖本身无法同时满足时返回 `ErrInfeasible`。承诺工期不足只产生负总时差，不报错。
- 结果与活动、依赖的输入排列无关。

## 测试

```sh
go test ./...
```

覆盖：正负时距 SS 闭环（最早 0/3、最晚 -1/2、总时差 -1，时距改 -2 后报依赖矛盾）、四种关系 + 负时距 + 零工期 + 独立子计划混合、修改后再改回数值恢复、输入排列不变性、参数校验。
