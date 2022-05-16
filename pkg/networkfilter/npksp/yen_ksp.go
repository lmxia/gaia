package npksp

import (
	"fmt"
	"math"
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

// YenKShortestPaths returns the k-shortest loopless paths from s to t in g.
// YenKShortestPaths will panic if g contains a negative edge weight.
func YenKShortestPaths(g graph.Graph, k int, s, t graph.Node) [][]graph.Node {
	// See https://en.wikipedia.org/wiki/Yen's_algorithm and
	// the paper at https://doi.org/10.1090%2Fqam%2F253822.

	_, isDirected := g.(graph.Directed)
	yk := yenKSPAdjuster{
		Graph:      g,
		isDirected: isDirected,
	}

	if wg, ok := g.(Weighted); ok {
		yk.weight = wg.Weight
	} else {
		yk.weight = UniformCost(g)
	}

	shortest, _ := DijkstraFrom(s, yk).To(t.ID())
	fmt.Printf("shortest  is:%+v\n", shortest)
	switch len(shortest) {
	case 0:
		return nil
	case 1:
		return [][]graph.Node{shortest}
	}
	paths := [][]graph.Node{shortest}
	for _, path := range paths {
		fmt.Printf("Path origin is:%+v\n", path)
	}
	var pot []YenShortest
	var root []graph.Node
	for i := int64(1); i < int64(k); i++ {
		for n := 0; n < len(paths[i-1])-2; n++ {
			yk.reset()

			spur := paths[i-1][n]
			root := append(root[:0], paths[i-1][:n+1]...)

			for _, path := range paths {
				if len(path) <= n {
					continue
				}
				ok := true
				for x := 0; x < len(root); x++ {
					if path[x].ID() != root[x].ID() {
						ok = false
						break
					}
				}
				if ok {
					yk.removeEdge(path[n].ID(), path[n+1].ID())
				}
			}

			spath, weight := DijkstraFrom(spur, yk).To(t.ID())
			if len(root) > 1 {
				var rootWeight float64
				for x := 1; x < len(root); x++ {
					w, _ := yk.weight(root[x-1].ID(), root[x].ID())
					rootWeight += w
				}
				root = append(root[:len(root)-1], spath...)
				pot = append(pot, YenShortest{root, weight + rootWeight})
			} else {
				pot = append(pot, YenShortest{spath, weight})
			}
		}

		if len(pot) == 0 {
			break
		}

		sort.Sort(ByPathWeight(pot))
		best := pot[0].Path

		if len(best) <= 1 {
			break
		}
		fmt.Printf("best Path is:%+v, weight is: %+v\n", best, pot[0].Weight)
		paths = append(paths, best)
		pot = pot[1:]
	}
	for _, path := range paths {
		fmt.Printf("YenKShortestPaths Path is:%+v\n", path)
	}
	return paths
}

// YenShortest holds a path and its weight for sorting.
type YenShortest struct {
	Path   []graph.Node
	Weight float64
}

type ByPathWeight []YenShortest

func (s ByPathWeight) Len() int           { return len(s) }
func (s ByPathWeight) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ByPathWeight) Less(i, j int) bool { return s[i].Weight < s[j].Weight }

// yenKSPAdjuster allows walked edges to be omitted from a graph
// without altering the embedded graph.
type yenKSPAdjuster struct {
	graph.Graph
	isDirected bool

	// weight is the edge weight function
	// used for shortest path calculation.
	weight Weighting

	// visitedEdges holds the edges that have
	// been removed by Yen's algorithm.
	visitedEdges map[[2]int64]struct{}
}

func (g yenKSPAdjuster) From(id int64) graph.Nodes {
	nodes := graph.NodesOf(g.Graph.From(id))
	for i := 0; i < len(nodes); {
		if g.canWalk(id, nodes[i].ID()) {
			i++
			continue
		}
		nodes[i] = nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
	}
	return iterator.NewOrderedNodes(nodes)
}

func (g yenKSPAdjuster) canWalk(u, v int64) bool {
	_, ok := g.visitedEdges[[2]int64{u, v}]
	return !ok
}

func (g yenKSPAdjuster) removeEdge(u, v int64) {
	g.visitedEdges[[2]int64{u, v}] = struct{}{}
	if g.isDirected {
		g.visitedEdges[[2]int64{v, u}] = struct{}{}
	}
}

func (g *yenKSPAdjuster) reset() {
	g.visitedEdges = make(map[[2]int64]struct{})
}

func (g yenKSPAdjuster) Weight(xid, yid int64) (w float64, ok bool) {
	return g.weight(xid, yid)
}

func Bipartite(n int, weight, inc float64) []simple.WeightedEdge {
	var edges []simple.WeightedEdge
	for i := 2; i < n+2; i++ {
		edges = append(edges,
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(i), W: weight},
			simple.WeightedEdge{F: simple.Node(i), T: simple.Node(1), W: weight},
		)
		weight += inc
	}
	return edges
}

func PathIDs(paths [][]graph.Node) [][]int64 {
	if paths == nil {
		return nil
	}
	ids := make([][]int64, len(paths))
	for i, p := range paths {
		if p == nil {
			continue
		}
		ids[i] = make([]int64, len(p))
		for j, n := range p {
			ids[i][j] = n.ID()
		}
	}
	return ids
}

func PathWeight(path []graph.Node, g graph.Weighted) float64 {
	switch len(path) {
	case 0:
		return math.NaN()
	case 1:
		return 0
	default:
		var w float64
		for i, u := range path[:len(path)-1] {
			_w, _ := g.Weight(u.ID(), path[i+1].ID())
			w += _w
		}
		return w
	}
}

func RemoveDuplicateElement(slc []int64) []int64 {
	var result []int64
	tempMap := map[int64]byte{} // 存放不重复主键
	for _, e := range slc {
		l := len(tempMap)
		tempMap[e] = 0
		if len(tempMap) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, e)
		}
	}
	return result
}
