package schedfloat

import (
	"errors"
	"testing"
)

func mustAddActivity(t *testing.T, p *Plan, id string, dur int64) {
	t.Helper()
	if err := p.AddActivity(Activity{ID: id, Duration: dur}); err != nil {
		t.Fatalf("AddActivity(%s): %v", id, err)
	}
}

func mustAddDep(t *testing.T, p *Plan, pred, succ string, typ RelationType, lag int64) {
	t.Helper()
	if err := p.AddDependency(Dependency{Predecessor: pred, Successor: succ, Type: typ, Lag: lag}); err != nil {
		t.Fatalf("AddDependency(%s->%s): %v", pred, succ, err)
	}
}

// 场景一：正负时距相互依赖不构成矛盾；T 不足只产生负总时差。
// A 工期 2、B 工期 1，A->B SS+3，B->A SS-3，T=3。
func TestScenario1_NegativeLagLoop(t *testing.T) {
	p := NewPlan(3)
	mustAddActivity(t, p, "A", 2)
	mustAddActivity(t, p, "B", 1)
	mustAddDep(t, p, "A", "B", StartToStart, 3)
	mustAddDep(t, p, "B", "A", StartToStart, -3)

	res, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	a := res.Activities["A"]
	b := res.Activities["B"]
	if a.EarliestStart != 0 || b.EarliestStart != 3 {
		t.Errorf("最早开始: got A=%d B=%d, want A=0 B=3", a.EarliestStart, b.EarliestStart)
	}
	if a.EarliestFinish != 2 || b.EarliestFinish != 4 {
		t.Errorf("最早完成: got A=%d B=%d, want A=2 B=4", a.EarliestFinish, b.EarliestFinish)
	}
	if res.ProjectEarliestFinish != 4 {
		t.Errorf("项目最早完工: got %d, want 4", res.ProjectEarliestFinish)
	}
	if a.LatestStart != -1 || b.LatestStart != 2 {
		t.Errorf("最晚开始: got A=%d B=%d, want A=-1 B=2", a.LatestStart, b.LatestStart)
	}
	if a.LatestFinish != 1 || b.LatestFinish != 3 {
		t.Errorf("最晚完成: got A=%d B=%d, want A=1 B=3", a.LatestFinish, b.LatestFinish)
	}
	if a.TotalFloat != -1 || b.TotalFloat != -1 {
		t.Errorf("总时差: got A=%d B=%d, want 均为 -1", a.TotalFloat, b.TotalFloat)
	}
	if !a.Critical || !b.Critical {
		t.Errorf("总时差 <= 0 的活动应标为关键活动")
	}
}

// 场景一后半：B->A 时距改为 -2 后依赖矛盾。
func TestScenario1_ConflictAfterLagChange(t *testing.T) {
	p := NewPlan(3)
	mustAddActivity(t, p, "A", 2)
	mustAddActivity(t, p, "B", 1)
	mustAddDep(t, p, "A", "B", StartToStart, 3)
	mustAddDep(t, p, "B", "A", StartToStart, -3)

	if err := p.SetLag(1, -2); err != nil {
		t.Fatalf("SetLag: %v", err)
	}
	_, err := Analyze(p)
	var conflict *DependencyConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("应报告依赖矛盾, got %v", err)
	}
}

// 场景二：四种关系、负时距、零工期里程碑与独立子计划混合。
func buildMixedPlan(t *testing.T) *Plan {
	t.Helper()
	p := NewPlan(20)
	mustAddActivity(t, p, "M", 0) // 零工期里程碑
	mustAddActivity(t, p, "A", 4)
	mustAddActivity(t, p, "B", 3)
	mustAddActivity(t, p, "C", 2)
	mustAddActivity(t, p, "D", 5)
	mustAddActivity(t, p, "E", 0)                 // 独立子计划中的零工期里程碑
	mustAddDep(t, p, "M", "A", FinishToStart, 0)  // 0: sA >= sM
	mustAddDep(t, p, "A", "B", StartToStart, 2)   // 1: sB >= sA+2
	mustAddDep(t, p, "A", "C", FinishToFinish, 1) // 2: sC >= sA+3
	mustAddDep(t, p, "B", "C", StartToFinish, -1) // 3: sC >= sB-3（负时距）
	mustAddDep(t, p, "D", "E", FinishToStart, -2) // 4: sE >= sD+3（负时距）
	mustAddDep(t, p, "M", "D", StartToStart, 1)   // 5: sD >= sM+1
	return p
}

func TestScenario2_MixedRelations(t *testing.T) {
	res, err := Analyze(buildMixedPlan(t))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	wantES := map[string]int64{"M": 0, "A": 0, "B": 2, "C": 3, "D": 1, "E": 4}
	wantLS := map[string]int64{"M": 14, "A": 15, "B": 17, "C": 18, "D": 15, "E": 20}
	for id, es := range wantES {
		got := res.Activities[id]
		if got.EarliestStart != es {
			t.Errorf("%s 最早开始: got %d, want %d", id, got.EarliestStart, es)
		}
		if got.LatestStart != wantLS[id] {
			t.Errorf("%s 最晚开始: got %d, want %d", id, got.LatestStart, wantLS[id])
		}
	}
	if res.ProjectEarliestFinish != 6 {
		t.Errorf("项目最早完工: got %d, want 6", res.ProjectEarliestFinish)
	}
	// 里程碑 M：完成时点等于开始时点。
	m := res.Activities["M"]
	if m.EarliestFinish != m.EarliestStart || m.LatestFinish != m.LatestStart {
		t.Errorf("零工期里程碑完成时点应等于开始时点: %+v", m)
	}
}

// 修改后再改回，排程数值必须恢复；修改结果必须与重新提交完整计划一致。
func TestModifyAndRevert(t *testing.T) {
	p := buildMixedPlan(t)
	before, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// 修改：A 工期 4->6，A->B 时距 2->5。
	if err := p.SetDuration("A", 6); err != nil {
		t.Fatalf("SetDuration: %v", err)
	}
	if err := p.SetLag(1, 5); err != nil {
		t.Fatalf("SetLag: %v", err)
	}
	modified, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze(修改后): %v", err)
	}

	// 重新提交等价的完整计划。
	fresh := NewPlan(20)
	mustAddActivity(t, fresh, "M", 0)
	mustAddActivity(t, fresh, "A", 6)
	mustAddActivity(t, fresh, "B", 3)
	mustAddActivity(t, fresh, "C", 2)
	mustAddActivity(t, fresh, "D", 5)
	mustAddActivity(t, fresh, "E", 0)
	mustAddDep(t, fresh, "M", "A", FinishToStart, 0)
	mustAddDep(t, fresh, "A", "B", StartToStart, 5)
	mustAddDep(t, fresh, "A", "C", FinishToFinish, 1)
	mustAddDep(t, fresh, "B", "C", StartToFinish, -1)
	mustAddDep(t, fresh, "D", "E", FinishToStart, -2)
	mustAddDep(t, fresh, "M", "D", StartToStart, 1)
	freshRes, err := Analyze(fresh)
	if err != nil {
		t.Fatalf("Analyze(完整计划): %v", err)
	}
	assertEqualResult(t, modified, freshRes)

	// 改回原值，排程数值必须恢复。
	if err := p.SetDuration("A", 4); err != nil {
		t.Fatalf("SetDuration: %v", err)
	}
	if err := p.SetLag(1, 2); err != nil {
		t.Fatalf("SetLag: %v", err)
	}
	reverted, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze(改回后): %v", err)
	}
	assertEqualResult(t, reverted, before)
}

func assertEqualResult(t *testing.T, got, want *Result) {
	t.Helper()
	if got.ProjectEarliestFinish != want.ProjectEarliestFinish {
		t.Errorf("项目最早完工: got %d, want %d", got.ProjectEarliestFinish, want.ProjectEarliestFinish)
	}
	if len(got.Activities) != len(want.Activities) {
		t.Fatalf("活动数量不一致: got %d, want %d", len(got.Activities), len(want.Activities))
	}
	for id, w := range want.Activities {
		if got.Activities[id] != w {
			t.Errorf("活动 %s: got %+v, want %+v", id, got.Activities[id], w)
		}
	}
}

// 输入排列（活动与依赖的添加顺序）不得改变排程数值。
func TestInputOrderDoesNotMatter(t *testing.T) {
	base, err := Analyze(buildMixedPlan(t))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	p := NewPlan(20)
	for _, a := range []Activity{{"E", 0}, {"D", 5}, {"C", 2}, {"B", 3}, {"A", 4}, {"M", 0}} {
		if err := p.AddActivity(a); err != nil {
			t.Fatalf("AddActivity: %v", err)
		}
	}
	for _, d := range []Dependency{
		{"M", "D", StartToStart, 1},
		{"D", "E", FinishToStart, -2},
		{"B", "C", StartToFinish, -1},
		{"A", "C", FinishToFinish, 1},
		{"A", "B", StartToStart, 2},
		{"M", "A", FinishToStart, 0},
	} {
		if err := p.AddDependency(d); err != nil {
			t.Fatalf("AddDependency: %v", err)
		}
	}
	shuffled, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze(乱序): %v", err)
	}
	assertEqualResult(t, shuffled, base)
}

// 可行的依赖闭环（含零工期环）不应误报矛盾。
func TestFeasibleCycle(t *testing.T) {
	p := NewPlan(10)
	mustAddActivity(t, p, "X", 0)
	mustAddActivity(t, p, "Y", 2)
	mustAddDep(t, p, "X", "Y", StartToStart, 1)
	mustAddDep(t, p, "Y", "X", StartToStart, -1) // 环：sX+1 <= sY <= sX+1
	res, err := Analyze(p)
	if err != nil {
		t.Fatalf("可行闭环不应报错: %v", err)
	}
	if res.Activities["X"].EarliestStart != 0 || res.Activities["Y"].EarliestStart != 1 {
		t.Errorf("最早开始: got X=%d Y=%d, want X=0 Y=1",
			res.Activities["X"].EarliestStart, res.Activities["Y"].EarliestStart)
	}
}

// 同一对活动允许多条依赖，且全部同时生效。
func TestMultipleDependenciesSamePair(t *testing.T) {
	p := NewPlan(10)
	mustAddActivity(t, p, "A", 1)
	mustAddActivity(t, p, "B", 1)
	mustAddDep(t, p, "A", "B", StartToStart, 2)
	mustAddDep(t, p, "A", "B", FinishToStart, 4) // 更紧：sB >= sA+5
	res, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.Activities["B"].EarliestStart != 5 {
		t.Errorf("B 最早开始: got %d, want 5（多条依赖同时满足）", res.Activities["B"].EarliestStart)
	}
}

// 输入校验：负工期、重复活动、悬空依赖、越界下标。
func TestValidation(t *testing.T) {
	p := NewPlan(5)
	if err := p.AddActivity(Activity{ID: "A", Duration: -1}); err == nil {
		t.Error("负工期应报错")
	}
	mustAddActivity(t, p, "A", 1)
	if err := p.AddActivity(Activity{ID: "A", Duration: 1}); err == nil {
		t.Error("重复活动应报错")
	}
	if err := p.AddDependency(Dependency{Predecessor: "A", Successor: "B"}); err == nil {
		t.Error("悬空依赖应报错")
	}
	if err := p.SetDuration("B", 1); err == nil {
		t.Error("修改不存在的活动应报错")
	}
	if err := p.SetLag(0, 1); err == nil {
		t.Error("越界依赖下标应报错")
	}
}
