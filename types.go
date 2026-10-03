// Package plansched 提供工程进度计划的最早/最晚排程与总时差分析。
// 计划与计算结果均保存在进程内存中，由上层直接调用。
package plansched

import "errors"

// ErrInfeasible 表示依赖关系本身无法同时满足（存在不可行的依赖闭环）。
// 承诺工期不足不属于此错误，只会产生负总时差。
var ErrInfeasible = errors.New("plansched: 依赖矛盾，无法同时满足所有依赖")

// Relation 为依赖关系类型。
type Relation int

const (
	// FinishToStart 后项开始不得早于前项完成加 L：Sj >= Si + Di + L
	FinishToStart Relation = iota
	// StartToStart 后项开始不得早于前项开始加 L：Sj >= Si + L
	StartToStart
	// FinishToFinish 后项完成不得早于前项完成加 L：Sj >= Si + Di - Dj + L
	FinishToFinish
	// StartToFinish 后项完成不得早于前项开始加 L：Sj >= Si + L - Dj
	StartToFinish
)

// Activity 为一项活动。工期非负，允许零工期里程碑。
type Activity struct {
	ID       string
	Duration int64 // 相对项目起点的整数工作小时
}

// Dependency 从前项指向后项，带整数时距 L（可为负）。
// 同一对活动允许存在多条依赖，用 ID 区分以便修改。
type Dependency struct {
	ID          string
	Predecessor string
	Successor   string
	Type        Relation
	Lag         int64
}

// Plan 为一份完整计划：活动、依赖与承诺完工时点 T。
type Plan struct {
	Activities   []Activity
	Dependencies []Dependency
	Deadline     int64 // 承诺完工时点 T
}

// ActivityResult 为单项活动的分析结果，均为整数工作小时。
type ActivityResult struct {
	ID             string
	EarliestStart  int64 // 所有可行计划中的最早开始
	EarliestFinish int64 // 最早完成 = 最早开始 + 工期
	LatestStart    int64 // 满足全部依赖且不晚于 T 完工的最晚开始（可为负）
	LatestFinish   int64 // 最晚完成 = 最晚开始 + 工期
	TotalFloat     int64 // 总时差 = 最晚开始 - 最早开始；负值表示赶工缺口
}

// Result 为整份计划的分析结果。
type Result struct {
	Activities            map[string]ActivityResult
	ProjectEarliestFinish int64    // 项目最早完工时点 = 各活动最早完成的最大值
	NoFloat               []string // 总时差不大于 0 的活动 ID（按 ID 排序）
}
