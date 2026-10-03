package plansched

import (
	"fmt"
	"sort"
)

// edge 为差分约束 Sj >= Si + w 中的一条边 i -> j。
type edge struct {
	from, to int
	w        int64
}

// buildEdges 把四种依赖关系统一折算为关于开始时点的差分约束。
func buildEdges(p Plan) ([]edge, error) {
	idx := make(map[string]int, len(p.Activities))
	for i, a := range p.Activities {
		if a.Duration < 0 {
			return nil, fmt.Errorf("plansched: 活动 %q 工期为负", a.ID)
		}
		if _, dup := idx[a.ID]; dup {
			return nil, fmt.Errorf("plansched: 活动 ID %q 重复", a.ID)
		}
		idx[a.ID] = i
	}
	edges := make([]edge, 0, len(p.Dependencies))
	for _, d := range p.Dependencies {
		i, ok := idx[d.Predecessor]
		if !ok {
			return nil, fmt.Errorf("plansched: 依赖 %q 引用了未知前项 %q", d.ID, d.Predecessor)
		}
		j, ok := idx[d.Successor]
		if !ok {
			return nil, fmt.Errorf("plansched: 依赖 %q 引用了未知后项 %q", d.ID, d.Successor)
		}
		di, dj := p.Activities[i].Duration, p.Activities[j].Duration
		var w int64
		switch d.Type {
		case FinishToStart:
			w = di + d.Lag
		case StartToStart:
			w = d.Lag
		case FinishToFinish:
			w = di - dj + d.Lag
		case StartToFinish:
			w = d.Lag - dj
		default:
			return nil, fmt.Errorf("plansched: 依赖 %q 关系类型非法", d.ID)
		}
		edges = append(edges, edge{from: i, to: j, w: w})
	}
	return edges, nil
}

// earliest 求每项活动在所有可行计划中的最早开始（开始不早于 0）。
// 第 n 轮仍可松弛说明存在正环，即依赖矛盾。
func earliest(n int, edges []edge) ([]int64, error) {
	dist := make([]int64, n) // 初始为 0，等价于超级源 Sj >= 0
	for round := 0; round < n; round++ {
		changed := false
		for _, e := range edges {
			if v := dist[e.from] + e.w; v > dist[e.to] {
				dist[e.to] = v
				changed = true
			}
		}
		if !changed {
			return dist, nil
		}
		if round == n-1 {
			return nil, ErrInfeasible
		}
	}
	return dist, nil
}

// latest 求每项活动满足全部依赖且不晚于 T 完工的最晚开始。
// 只收紧上界，不截断负值。
func latest(p Plan, edges []edge) ([]int64, error) {
	n := len(p.Activities)
	ub := make([]int64, n)
	for i, a := range p.Activities {
		ub[i] = p.Deadline - a.Duration
	}
	for round := 0; round < n; round++ {
		changed := false
		for _, e := range edges {
			if v := ub[e.to] - e.w; v < ub[e.from] {
				ub[e.from] = v
				changed = true
			}
		}
		if !changed {
			return ub, nil
		}
		if round == n-1 {
			return nil, ErrInfeasible
		}
	}
	return ub, nil
}

// Analyze 对整份计划做最早/最晚排程与总时差分析。
// 依赖本身无法同时满足时返回 ErrInfeasible；承诺工期不足只产生负总时差。
// 结果与活动、依赖的输入排列无关。
func Analyze(p Plan) (Result, error) {
	edges, err := buildEdges(p)
	if err != nil {
		return Result{}, err
	}
	n := len(p.Activities)
	res := Result{Activities: make(map[string]ActivityResult, n)}
	if n == 0 {
		return res, nil
	}
	es, err := earliest(n, edges)
	if err != nil {
		return Result{}, err
	}
	ls, err := latest(p, edges)
	if err != nil {
		return Result{}, err
	}
	for i, a := range p.Activities {
		r := ActivityResult{
			ID:             a.ID,
			EarliestStart:  es[i],
			EarliestFinish: es[i] + a.Duration,
			LatestStart:    ls[i],
			LatestFinish:   ls[i] + a.Duration,
			TotalFloat:     ls[i] - es[i],
		}
		res.Activities[a.ID] = r
		if r.EarliestFinish > res.ProjectEarliestFinish {
			res.ProjectEarliestFinish = r.EarliestFinish
		}
		if r.TotalFloat <= 0 {
			res.NoFloat = append(res.NoFloat, a.ID)
		}
	}
	sort.Strings(res.NoFloat)
	return res, nil
}
