# schedfloat — 工程进度与总时差分析库

Go 实现的进程内进度计划分析库。支持四种搭接关系（FS/SS/FF/SF）、正负整数时距、
零工期里程碑与依赖闭环，输出最早/最晚排程、总时差与关键活动标记，
并将「依赖本身矛盾」与「承诺工期不足」严格区分。

## 模型约定

- 时间为相对项目起点的整数工作小时；完成时点 = 开始时点 + 工期。
- 依赖从前项指向后项，时距 `Lag` 为整数（可为负），同一对活动允许多条依赖，全部同时生效。
- 最早计划：所有活动开始不早于 0 且满足全部依赖。
- 最晚计划：全部活动完成不晚于承诺时点 T，允许开始早于 0（不截断负值）。
- 总时差 = 最晚开始 − 最早开始；≤ 0 的活动标记为关键（`Critical`）。
- 依赖约束存在正权闭环时报 `DependencyConflictError`；T 不足只产生负总时差，不报错。

## 调用示例

```go
package main

import (
	"errors"
	"fmt"

	"schedfloat"
)

func main() {
	p := schedfloat.NewPlan(3) // 承诺完工时点 T=3
	p.AddActivity(schedfloat.Activity{ID: "A", Duration: 2})
	p.AddActivity(schedfloat.Activity{ID: "B", Duration: 1})
	p.AddDependency(schedfloat.Dependency{
		Predecessor: "A", Successor: "B",
		Type: schedfloat.StartToStart, Lag: 3,
	})
	p.AddDependency(schedfloat.Dependency{
		Predecessor: "B", Successor: "A",
		Type: schedfloat.StartToStart, Lag: -3,
	})

	res, err := schedfloat.Analyze(p)
	var conflict *schedfloat.DependencyConflictError
	if errors.As(err, &conflict) {
		fmt.Println("依赖矛盾，需调整搭接关系")
		return
	}
	for id, a := range res.Activities {
		fmt.Printf("%s: ES=%d EF=%d LS=%d LF=%d TF=%d critical=%v\n",
			id, a.EarliestStart, a.EarliestFinish,
			a.LatestStart, a.LatestFinish, a.TotalFloat, a.Critical)
	}
	fmt.Println("项目最早完工:", res.ProjectEarliestFinish)

	// 修改后重新分析，结果与重新提交完整计划一致。
	p.SetDuration("A", 3)
	p.SetLag(1, -2) // 下标按依赖添加顺序，从 0 开始
	res, err = schedfloat.Analyze(p)
	_ = res
}
```

## API 一览

- `NewPlan(t int64) *Plan`：创建计划，`t` 为承诺完工时点。
- `(*Plan).AddActivity(Activity) error`：添加活动（工期非负，ID 唯一）。
- `(*Plan).AddDependency(Dependency) error`：添加依赖（引用的活动须已存在）。
- `(*Plan).SetDuration(id string, duration int64) error`：修改活动工期。
- `(*Plan).SetLag(index int, lag int64) error`：按添加顺序修改依赖时距。
- `Analyze(*Plan) (*Result, error)`：整份计划分析；每次调用独立全量计算，
  因此修改后重算与重新提交完整计划等价，且输入排列不影响数值。

## 算法

将四种关系统一转换为开始时间的差分约束 `s_v >= s_u + w`
（FS: `w=d_u+L`，SS: `w=L`，FF: `w=d_u+L−d_v`，SF: `w=L−d_v`），
用 Bellman-Ford 前推求最早开始（同时检测正权闭环即依赖矛盾），
再以 `l_i ≤ T−d_i` 逆推求最晚开始。复杂度 O(活动数 × 依赖数)，整数精确运算。

## 测试

```sh
go test ./...
```

覆盖：正负时距互赖与矛盾识别、四种关系混合、负时距、零工期里程碑、
独立子计划、修改—恢复一致性、修改与重新提交等价性、输入排列无关性、
可行闭环、同对活动多依赖、输入校验。
