package plansched

import (
	"errors"
	"reflect"
	"testing"
)

// 场景二：四种关系 + 负时距 + 零工期里程碑 + 独立子计划混合。
// M(0h) 里程碑；C(4h)、D(3h)、E(2h) 主链；X(5h)->Y(1h) 独立子计划。T=6。
func mixedPlan() Plan {
	return Plan{
		Activities: []Activity{
			{ID: "M", Duration: 0},
			{ID: "C", Duration: 4},
			{ID: "D", Duration: 3},
			{ID: "E", Duration: 2},
			{ID: "X", Duration: 5},
			{ID: "Y", Duration: 1},
		},
		Dependencies: []Dependency{
			{ID: "fs", Predecessor: "M", Successor: "C", Type: FinishToStart, Lag: 0},
			{ID: "ss", Predecessor: "C", Successor: "D", Type: StartToStart, Lag: -1},
			{ID: "ff", Predecessor: "C", Successor: "E", Type: FinishToFinish, Lag: 2},
			{ID: "sf", Predecessor: "D", Successor: "E", Type: StartToFinish, Lag: 1},
			{ID: "xy", Predecessor: "X", Successor: "Y", Type: FinishToStart, Lag: -2},
		},
		Deadline: 6,
	}
}

func TestMixedPlan(t *testing.T) {
	res, err := Analyze(mixedPlan())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := map[string]ActivityResult{
		"M": {ID: "M", EarliestStart: 0, EarliestFinish: 0, LatestStart: 0, LatestFinish: 0, TotalFloat: 0},
		"C": {ID: "C", EarliestStart: 0, EarliestFinish: 4, LatestStart: 0, LatestFinish: 4, TotalFloat: 0},
		"D": {ID: "D", EarliestStart: 0, EarliestFinish: 3, LatestStart: 3, LatestFinish: 6, TotalFloat: 3},
		"E": {ID: "E", EarliestStart: 4, EarliestFinish: 6, LatestStart: 4, LatestFinish: 6, TotalFloat: 0},
		"X": {ID: "X", EarliestStart: 0, EarliestFinish: 5, LatestStart: 1, LatestFinish: 6, TotalFloat: 1},
		"Y": {ID: "Y", EarliestStart: 3, EarliestFinish: 4, LatestStart: 5, LatestFinish: 6, TotalFloat: 2},
	}
	if !reflect.DeepEqual(res.Activities, want) {
		for id, w := range want {
			if got := res.Activities[id]; got != w {
				t.Errorf("活动 %s: got %+v, want %+v", id, got, w)
			}
		}
	}
	if res.ProjectEarliestFinish != 6 {
		t.Errorf("项目最早完工: got %d, want 6", res.ProjectEarliestFinish)
	}
	if !reflect.DeepEqual(res.NoFloat, []string{"C", "E", "M"}) {
		t.Errorf("无时差活动: got %v, want [C E M]", res.NoFloat)
	}
}

// 修改工期后再改回，排程数值必须恢复；且与重新提交完整计划一致。
func TestModifyAndRevert(t *testing.T) {
	s, err := NewSchedule(mixedPlan())
	if err != nil {
		t.Fatalf("NewSchedule: %v", err)
	}
	original := s.Result()

	// C 工期 4 -> 6：最晚计划允许开始早于 0，不得截断负值。
	if err := s.SetDuration("C", 6); err != nil {
		t.Fatalf("SetDuration: %v", err)
	}
	got := s.Result()
	if got.ProjectEarliestFinish != 8 {
		t.Errorf("项目最早完工: got %d, want 8", got.ProjectEarliestFinish)
	}
	if got.Activities["C"].LatestStart != -2 || got.Activities["M"].LatestStart != -2 {
		t.Errorf("最晚开始: got C=%d M=%d, want C=-2 M=-2",
			got.Activities["C"].LatestStart, got.Activities["M"].LatestStart)
	}
	if got.Activities["C"].TotalFloat != -2 {
		t.Errorf("C 总时差: got %d, want -2", got.Activities["C"].TotalFloat)
	}

	// 与重新提交完整计划（C 工期 6）的结果一致。
	p := mixedPlan()
	p.Activities[1].Duration = 6
	fresh, err := Analyze(p)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !reflect.DeepEqual(got, fresh) {
		t.Errorf("修改后结果与重新提交完整计划不一致:\n got %+v\nwant %+v", got, fresh)
	}

	// 改回 4，排程数值必须恢复。
	if err := s.SetDuration("C", 4); err != nil {
		t.Fatalf("SetDuration revert: %v", err)
	}
	if !reflect.DeepEqual(s.Result(), original) {
		t.Errorf("改回后结果未恢复:\n got %+v\nwant %+v", s.Result(), original)
	}

	// 修改时距再改回，同样必须恢复。
	if err := s.SetLag("ss", 2); err != nil {
		t.Fatalf("SetLag: %v", err)
	}
	if err := s.SetLag("ss", -1); err != nil {
		t.Fatalf("SetLag revert: %v", err)
	}
	if !reflect.DeepEqual(s.Result(), original) {
		t.Errorf("时距改回后结果未恢复:\n got %+v\nwant %+v", s.Result(), original)
	}
}

// 修改导致依赖矛盾时返回错误并回滚，Schedule 保持修改前状态。
func TestInfeasibleMutationRollsBack(t *testing.T) {
	s, err := NewSchedule(ssCyclePlan())
	if err != nil {
		t.Fatalf("NewSchedule: %v", err)
	}
	before := s.Result()
	if err := s.SetLag("d2", -2); !errors.Is(err, ErrInfeasible) {
		t.Fatalf("SetLag: got %v, want ErrInfeasible", err)
	}
	if !reflect.DeepEqual(s.Result(), before) {
		t.Errorf("回滚后结果不一致:\n got %+v\nwant %+v", s.Result(), before)
	}
	if got := s.Plan().Dependencies[1].Lag; got != -3 {
		t.Errorf("回滚后时距: got %d, want -3", got)
	}
}

// 未知 ID 报错。
func TestUnknownIDs(t *testing.T) {
	s, err := NewSchedule(mixedPlan())
	if err != nil {
		t.Fatalf("NewSchedule: %v", err)
	}
	if err := s.SetDuration("ZZ", 1); err == nil {
		t.Error("SetDuration 未知活动应报错")
	}
	if err := s.SetLag("ZZ", 1); err == nil {
		t.Error("SetLag 未知依赖应报错")
	}
}
