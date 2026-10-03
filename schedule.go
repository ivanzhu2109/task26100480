package plansched

import "fmt"

// Schedule 持有一份计划及其最新分析结果，支持修改后重新分析。
// 每次修改都基于完整计划从头计算，因此结果与重新提交完整计划一致。
type Schedule struct {
	plan   Plan
	result Result
}

// NewSchedule 提交完整计划并完成首次分析。
func NewSchedule(p Plan) (*Schedule, error) {
	s := &Schedule{plan: p}
	if err := s.reanalyze(); err != nil {
		return nil, err
	}
	return s, nil
}

// Result 返回当前最新分析结果。
func (s *Schedule) Result() Result {
	return s.result
}

// Plan 返回当前持有的计划副本，便于上层核对或持久化。
func (s *Schedule) Plan() Plan {
	p := s.plan
	p.Activities = append([]Activity(nil), p.Activities...)
	p.Dependencies = append([]Dependency(nil), p.Dependencies...)
	return p
}

// SetDuration 修改活动工期并重新分析。
// 若修改导致依赖矛盾，计划回滚到修改前并返回错误。
func (s *Schedule) SetDuration(id string, duration int64) error {
	for i := range s.plan.Activities {
		if s.plan.Activities[i].ID == id {
			old := s.plan.Activities[i].Duration
			s.plan.Activities[i].Duration = duration
			if err := s.reanalyze(); err != nil {
				s.plan.Activities[i].Duration = old
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("plansched: 未知活动 %q", id)
}

// SetLag 修改依赖时距并重新分析。
// 若修改导致依赖矛盾，计划回滚到修改前并返回错误。
func (s *Schedule) SetLag(depID string, lag int64) error {
	for i := range s.plan.Dependencies {
		if s.plan.Dependencies[i].ID == depID {
			old := s.plan.Dependencies[i].Lag
			s.plan.Dependencies[i].Lag = lag
			if err := s.reanalyze(); err != nil {
				s.plan.Dependencies[i].Lag = old
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("plansched: 未知依赖 %q", depID)
}

func (s *Schedule) reanalyze() error {
	r, err := Analyze(s.plan)
	if err != nil {
		return err
	}
	s.result = r
	return nil
}
