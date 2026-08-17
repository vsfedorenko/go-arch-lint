package checker

import "sort"

// Pure graph algorithms shared by graph-based checkers. No dependency on
// arch.Spec or project files — operate on the componentGraph adjacency
// structure only.

// findSCCs returns all strongly connected components of size > 1.
// Iterative Tarjan — no recursion, so deep graphs cannot blow the stack.
// Deterministic: nodes and successors are visited in sorted order.
func findSCCs(graph componentGraph) [][]string {
	nodes := sortedGraphNodes(graph)

	type frame struct {
		v          string
		successors []string
		next       int
	}

	index := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	var sccs [][]string
	next := 0

	sortedSuccessors := func(v string) []string {
		succ := make([]string, 0, len(graph[v]))
		for w := range graph[v] {
			succ = append(succ, w)
		}
		sort.Strings(succ)
		return succ
	}

	for _, root := range nodes {
		if _, seen := index[root]; seen {
			continue
		}

		frames := []frame{{v: root}}

		for len(frames) > 0 {
			f := &frames[len(frames)-1]

			if f.next == 0 {
				index[f.v] = next
				lowlink[f.v] = next
				next++
				stack = append(stack, f.v)
				onStack[f.v] = true

				f.successors = sortedSuccessors(f.v)
			}

			if f.next < len(f.successors) {
				w := f.successors[f.next]
				f.next++

				if _, seen := index[w]; !seen {
					frames = append(frames, frame{v: w})
				} else if onStack[w] && index[w] < lowlink[f.v] {
					lowlink[f.v] = index[w]
				}

				continue
			}

			// All successors processed — finish v.
			if lowlink[f.v] == index[f.v] {
				var scc []string
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					scc = append(scc, w)
					if w == f.v {
						break
					}
				}
				if len(scc) > 1 {
					sccs = append(sccs, scc)
				}
			}

			frames = frames[:len(frames)-1]

			if len(frames) > 0 {
				parent := &frames[len(frames)-1]
				if lowlink[f.v] < lowlink[parent.v] {
					lowlink[parent.v] = lowlink[f.v]
				}
			}
		}
	}

	return sccs
}

// orderCycle rotates an SCC into traversal order: starting from the
// smallest node, repeatedly move to the smallest unvisited in-SCC
// successor. For a simple cycle this yields the actual hop sequence; for
// complex SCCs it yields a walk visiting every node, with unvisited
// nodes appended in sorted order.
func orderCycle(scc []string, graph componentGraph) []string {
	sorted := append([]string(nil), scc...)
	sort.Strings(sorted)

	inSCC := make(map[string]struct{}, len(sorted))
	for _, n := range sorted {
		inSCC[n] = struct{}{}
	}

	ordered := make([]string, 0, len(sorted))
	visited := map[string]struct{}{}

	cur := sorted[0]
	for {
		if _, seen := visited[cur]; seen {
			break
		}
		visited[cur] = struct{}{}
		ordered = append(ordered, cur)

		next := ""
		for w := range graph[cur] {
			if _, isIn := inSCC[w]; !isIn {
				continue
			}
			if _, seen := visited[w]; seen {
				continue
			}
			if next == "" || w < next {
				next = w
			}
		}

		if next == "" {
			break
		}
		cur = next
	}

	for _, n := range sorted {
		if _, seen := visited[n]; !seen {
			ordered = append(ordered, n)
		}
	}

	return ordered
}
