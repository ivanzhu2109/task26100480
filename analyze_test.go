package plansched

import (
	"errors"
	"testing"
)

// 场景一：A(2h)、B(1h)，SS(A,B,L=3)，SS(B,A,L=-3)，T=3。
// 依赖闭环可行：最早开始 0/3，最晚开始 -1/2，总时差均 -1。
func ssCyclePlan() Plan {
	return Plan{
		Activities: []Activity{
			{ID: "A", Duration: 2},
			{ID: "B", Duration: 1},
		},
		Dependencies: []Dependency{
			{ID: "d1", Predecessor: "A", Successor: "B", Type: StartToStart, Lag: 3},
			{ID: "d2", Predecessor: "B", Successor: "A", Type: StartToStart, Lag: -3},
		},
		Deadline: 3,
	}
}

func TestSSCycleFeasible(t *testing.T) {
	res, err := Analyze(ssCyclePlan())
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
		t.Errorf("总时差: got A=%d B=%d, want A=-1 B=-1", a.TotalFloat, b.TotalFloat)
	}
	if len(res.NoFloat) != 2 || res.NoFloat[0] != "A" || res.NoFloat[1] != "B" {
		t.Errorf("无时差活动: got %v, want [A B]", res.NoFloat)
	}
}

// 场景一（续）：B->A 时距改为 -2 后闭环要求 S_A >= S_A + 1，依赖矛盾。
func TestSSCycleInfeasible(t *testing.T) {
	p := ssCyclePlan()
	p.Dependencies[1].Lag = -2
	_, err := Analyze(p)
	if !errors.Is(err, ErrInfeasible) {
		t.Fatalf("got %v, want ErrInfeasible", err)
	}
}

// 承诺工期不足只产生负总时差，不属于依赖矛盾。
func TestTightDeadlineIsNotContradiction(t *testing.T) {
	p := Plan{
		Activities: []Activity{{ID: "A", Duration: 10}},
		Deadline:   3,
	}
	res, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if res.Activities["A"].TotalFloat != -7 {
		t.Errorf("总时差: got %d, want -7", res.Activities["A"].TotalFloat)
	}
}

// 活动与依赖的输入排列不得改变排程数值。
func TestInputOrderDoesNotMatter(t *testing.T) {
	base := ssCyclePlan()
	permuted := Plan{
		Activities: []Activity{base.Activities[1], base.Activities[0]},
		Dependencies: []Dependency{
			base.Dependencies[1],
			base.Dependencies[0],
		},
		Deadline: base.Deadline,
	}
	r1, err1 := Analyze(base)
	r2, err2 := Analyze(permuted)
	if err1 != nil || err2 != nil {
		t.Fatalf("Analyze: %v / %v", err1, err2)
	}
	if r1.ProjectEarliestFinish != r2.ProjectEarliestFinish {
		t.Errorf("项目最早完工不一致: %d vs %d", r1.ProjectEarliestFinish, r2.ProjectEarliestFinish)
	}
	for id, a := range r1.Activities {
		if r2.Activities[id] != a {
			t.Errorf("活动 %s 结果不一致: %+v vs %+v", id, a, r2.Activities[id])
		}
	}
}

// 输入校验：负工期、重复 ID、未知引用。
func TestValidation(t *testing.T) {
	cases := []Plan{
		{Activities: []Activity{{ID: "A", Duration: -1}}},
		{Activities: []Activity{{ID: "A"}, {ID: "A"}}},
		{
			Activities:   []Activity{{ID: "A"}},
			Dependencies: []Dependency{{ID: "d", Predecessor: "A", Successor: "B", Type: StartToStart}},
		},
	}
	for i, p := range cases {
		if _, err := Analyze(p); err == nil || errors.Is(err, ErrInfeasible) {
			t.Errorf("case %d: got %v, want 校验错误", i, err)
		}
	}
}
