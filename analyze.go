package schedfloat

import "sort"

// edge 是约束图中的一条边：start[to] >= start[from] + w。
type edge struct {
	from, to string
	w        int64
}

// buildEdges 把依赖转换为关于开始时间的差分约束边。
// 完成时点 = 开始时点 + 工期，四种关系统一为 s_v >= s_u + w。
func buildEdges(p *Plan) []edge {
	edges := make([]edge, 0, len(p.deps))
	for _, d := range p.deps {
		du := p.activities[d.Predecessor]
		dv := p.activities[d.Successor]
		var w int64
		switch d.Type {
		case FinishToStart: // s_v >= s_u + d_u + L
			w = du + d.Lag
		case StartToStart: // s_v >= s_u + L
			w = d.Lag
		case FinishToFinish: // s_v + d_v >= s_u + d_u + L
			w = du + d.Lag - dv
		case StartToFinish: // s_v + d_v >= s_u + L
			w = d.Lag - dv
		}
		edges = append(edges, edge{from: d.Predecessor, to: d.Successor, w: w})
	}
	return edges
}

// sortedIDs 返回排序后的活动 ID，保证结果与输入排列无关。
func sortedIDs(p *Plan) []string {
	ids := make([]string, 0, len(p.activities))
	for id := range p.activities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// earliest 前推计算最早开始：s_i >= 0 且 s_v >= s_u + w。
// 用 Bellman-Ford 求最长路；若还能继续松弛则存在正权闭环，即依赖矛盾。
func earliest(ids []string, edges []edge) (map[string]int64, bool) {
	dist := make(map[string]int64, len(ids))
	for _, id := range ids {
		dist[id] = 0
	}
	for i := 0; i < len(ids); i++ {
		relaxed := false
		for _, e := range edges {
			if dist[e.from]+e.w > dist[e.to] {
				dist[e.to] = dist[e.from] + e.w
				relaxed = true
			}
		}
		if !relaxed {
			return dist, true
		}
	}
	// 第 n 轮仍可松弛：存在正权闭环。
	for _, e := range edges {
		if dist[e.from]+e.w > dist[e.to] {
			return nil, false
		}
	}
	return dist, true
}

// latest 逆推计算最晚开始：l_i <= T - d_i 且 l_u <= l_v - w。
// 允许负值，不截断。调用前须确认依赖可行。
func latest(p *Plan, ids []string, edges []edge) map[string]int64 {
	dist := make(map[string]int64, len(ids))
	for _, id := range ids {
		dist[id] = p.T - p.activities[id]
	}
	for i := 0; i < len(ids); i++ {
		relaxed := false
		for _, e := range edges {
			if dist[e.to]-e.w < dist[e.from] {
				dist[e.from] = dist[e.to] - e.w
				relaxed = true
			}
		}
		if !relaxed {
			break
		}
	}
	return dist
}

// Analyze 对整份计划做最早/最晚排程与总时差分析。
// 依赖矛盾时返回 *DependencyConflictError；承诺工期不足只产生负总时差。
func Analyze(p *Plan) (*Result, error) {
	ids := sortedIDs(p)
	edges := buildEdges(p)

	es, ok := earliest(ids, edges)
	if !ok {
		return nil, &DependencyConflictError{}
	}
	ls := latest(p, ids, edges)

	res := &Result{Activities: make(map[string]ActivityResult, len(ids))}
	for _, id := range ids {
		d := p.activities[id]
		tf := ls[id] - es[id]
		res.Activities[id] = ActivityResult{
			EarliestStart:  es[id],
			EarliestFinish: es[id] + d,
			LatestStart:    ls[id],
			LatestFinish:   ls[id] + d,
			TotalFloat:     tf,
			Critical:       tf <= 0,
		}
		if ef := es[id] + d; ef > res.ProjectEarliestFinish {
			res.ProjectEarliestFinish = ef
		}
	}
	return res, nil
}
