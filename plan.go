// Package schedfloat 提供工程进度计划的最早/最晚排程与总时差分析。
// 时间统一为相对项目起点的整数工作小时，全部状态保存在进程内存中。
package schedfloat

import "fmt"

// RelationType 表示依赖的搭接关系类型。
type RelationType int

const (
	// FinishToStart 后项开始不得早于前项完成加时距。
	FinishToStart RelationType = iota
	// StartToStart 后项开始不得早于前项开始加时距。
	StartToStart
	// FinishToFinish 后项完成不得早于前项完成加时距。
	FinishToFinish
	// StartToFinish 后项完成不得早于前项开始加时距。
	StartToFinish
)

// Activity 表示一项活动。工期非负，零工期表示里程碑。
type Activity struct {
	ID       string
	Duration int64
}

// Dependency 表示从前项 Predecessor 指向后项 Successor 的一条依赖，
// 带整数时距 Lag（可为负）。同一对活动允许存在多条依赖。
type Dependency struct {
	Predecessor string
	Successor   string
	Type        RelationType
	Lag         int64
}

// Plan 表示一份完整计划：活动集合、依赖集合与承诺完工时点 T。
type Plan struct {
	activities map[string]int64
	deps       []Dependency
	T          int64
}

// NewPlan 创建一份空计划，t 为承诺完工时点（相对项目起点的工作小时）。
func NewPlan(t int64) *Plan {
	return &Plan{activities: make(map[string]int64), T: t}
}

// AddActivity 添加活动；工期为负或 ID 重复时返回错误。
func (p *Plan) AddActivity(a Activity) error {
	if a.Duration < 0 {
		return fmt.Errorf("活动 %q 工期不能为负: %d", a.ID, a.Duration)
	}
	if _, ok := p.activities[a.ID]; ok {
		return fmt.Errorf("活动 %q 已存在", a.ID)
	}
	p.activities[a.ID] = a.Duration
	return nil
}

// AddDependency 添加依赖；引用不存在的活动返回错误。
func (p *Plan) AddDependency(d Dependency) error {
	if _, ok := p.activities[d.Predecessor]; !ok {
		return fmt.Errorf("依赖引用了不存在的前项活动 %q", d.Predecessor)
	}
	if _, ok := p.activities[d.Successor]; !ok {
		return fmt.Errorf("依赖引用了不存在的后项活动 %q", d.Successor)
	}
	p.deps = append(p.deps, d)
	return nil
}

// SetDuration 修改活动工期；修改后需重新调用 Analyze。
func (p *Plan) SetDuration(id string, duration int64) error {
	if duration < 0 {
		return fmt.Errorf("活动 %q 工期不能为负: %d", id, duration)
	}
	if _, ok := p.activities[id]; !ok {
		return fmt.Errorf("活动 %q 不存在", id)
	}
	p.activities[id] = duration
	return nil
}

// SetLag 修改第 index 条依赖（按添加顺序，从 0 开始）的时距；
// 修改后需重新调用 Analyze。
func (p *Plan) SetLag(index int, lag int64) error {
	if index < 0 || index >= len(p.deps) {
		return fmt.Errorf("依赖下标越界: %d", index)
	}
	p.deps[index].Lag = lag
	return nil
}

// ActivityResult 是单项活动的分析结果。
type ActivityResult struct {
	EarliestStart  int64
	EarliestFinish int64
	LatestStart    int64
	LatestFinish   int64
	TotalFloat     int64
	Critical       bool // 总时差 <= 0，表示没有延期余地
}

// Result 是整份计划的分析结果。
type Result struct {
	// Activities 按活动 ID 索引各项活动的排程结果。
	Activities map[string]ActivityResult
	// ProjectEarliestFinish 是所有活动最早完成时点的最大值。
	ProjectEarliestFinish int64
}

// DependencyConflictError 表示依赖本身无法同时满足（约束图中存在正权闭环）。
// 承诺工期不足只会产生负总时差，不会返回该错误。
type DependencyConflictError struct{}

func (e *DependencyConflictError) Error() string {
	return "依赖矛盾：依赖约束无法同时满足"
}
