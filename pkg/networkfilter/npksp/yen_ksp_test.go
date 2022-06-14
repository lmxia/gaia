package npksp

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/networkfilter/logx"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	"math"
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

var yenShortestPathTests = []struct {
	name  string
	graph func() graph.WeightedEdgeAdder
	edges []simple.WeightedEdge

	query     simple.Edge
	k         int
	wantPaths [][]int64

	relaxed bool
}{
	{
		// https://en.wikipedia.org/w/index.php?title=Yen%27s_algorithm&oldid=841018784#Example
		name:  "wikipedia example",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		/*edges: []simple.WeightedEdge{
			{F: simple.Node('C'), T: simple.Node('D'), W: 3},
			{F: simple.Node('C'), T: simple.Node('E'), W: 2},
			{F: simple.Node('E'), T: simple.Node('D'), W: 1},
			{F: simple.Node('D'), T: simple.Node('F'), W: 4},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('E'), T: simple.Node('G'), W: 3},
			{F: simple.Node('F'), T: simple.Node('G'), W: 2},
			{F: simple.Node('F'), T: simple.Node('H'), W: 1},
			{F: simple.Node('G'), T: simple.Node('H'), W: 2},
		},*/
		edges: []simple.WeightedEdge{
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 2},
			{F: simple.Node(3), T: simple.Node(4), W: 3},
			{F: simple.Node(1), T: simple.Node(3), W: 1},
			{F: simple.Node(4), T: simple.Node(3), W: 3},
			{F: simple.Node(3), T: simple.Node(2), W: 2},
			{F: simple.Node(2), T: simple.Node(1), W: 1},
			{F: simple.Node(4), T: simple.Node(2), W: 2},
		},
		/*edges: []simple.WeightedEdge{
			{F: simple.Node('C'), T: simple.Node('E'), W: 2},
			{F: simple.Node('C'), T: simple.Node('D'), W: 3},
			{F: simple.Node('E'), T: simple.Node('D'), W: 1},
			{F: simple.Node('E'), T: simple.Node('G'), W: 3},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('D'), T: simple.Node('F'), W: 4},
			{F: simple.Node('G'), T: simple.Node('H'), W: 2},
			{F: simple.Node('F'), T: simple.Node('G'), W: 2},
			{F: simple.Node('F'), T: simple.Node('H'), W: 1},
		},*/
		query: simple.Edge{F: simple.Node(1), T: simple.Node(4)},
		k:     5,
		/*wantPaths: [][]int64{
			{'C', 'E', 'F', 'H'},
			{'C', 'E', 'G', 'H'},
			{'C', 'D', 'F', 'H'},
		},*/
		wantPaths: [][]int64{
			{1, 3, 4},
			{1, 2, 3, 4},
		},
	},
	{
		name:  "1 edge graph",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 3},
		},
		query: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:     10,
		wantPaths: [][]int64{
			{0, 1},
		},
	},
	{
		name:      "empty graph",
		graph:     func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges:     []simple.WeightedEdge{},
		query:     simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:         1,
		wantPaths: nil,
	},
	{
		name:  "n-star graph",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 3},
			{F: simple.Node(0), T: simple.Node(2), W: 3},
			{F: simple.Node(0), T: simple.Node(3), W: 3},
		},
		query: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:     1,
		wantPaths: [][]int64{
			{0, 1},
		},
	},
	{
		name:  "bipartite small",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: Bipartite(5, 3, 0),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     10,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:  "bipartite parity",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: Bipartite(5, 3, 0),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:    "bipartite large",
		graph:   func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges:   Bipartite(10, 3, 0),
		query:   simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:       5,
		relaxed: true,
	},
	{
		name:  "bipartite inc",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: Bipartite(5, 10, 1),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:  "bipartite dec",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: Bipartite(5, 10, -1),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 6, 1},
			{-1, 5, 1},
			{-1, 4, 1},
			{-1, 3, 1},
			{-1, 2, 1},
		},
	},
	{
		name:  "waterfall", // This is the failing case in gonum/gonum#1700.
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(1), T: simple.Node(3), W: 1},
			{F: simple.Node(1), T: simple.Node(5), W: 1},
			{F: simple.Node(2), T: simple.Node(4), W: 1},
			{F: simple.Node(2), T: simple.Node(5), W: 1},
			{F: simple.Node(3), T: simple.Node(6), W: 1},
			{F: simple.Node(4), T: simple.Node(6), W: 1},
			{F: simple.Node(5), T: simple.Node(6), W: 1},
			{F: simple.Node(5), T: simple.Node(6), W: 1},
		},
		query: simple.Edge{F: simple.Node(0), T: simple.Node(6)},
		k:     4,
		wantPaths: [][]int64{
			{0, 1, 3, 6},
			{0, 1, 5, 6},
			{0, 1, 2, 4, 6},
			{0, 1, 2, 5, 6},
		},
	},
}

func TestYenKSP(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestYenKSP  BEGIN ===")
	nputil.TraceInfo(infoString)

	t.Parallel()
	for _, test := range yenShortestPathTests {
		g := test.graph()
		for _, e := range test.edges {
			g.SetWeightedEdge(e)
		}

		got := YenKShortestPaths(g.(graph.Graph), test.k, test.query.From(), test.query.To())
		gotIDs := PathIDs(got)

		paths := make(ByPathWeight, len(gotIDs))
		for i, p := range got {
			paths[i] = YenShortest{Path: p, Weight: PathWeight(p, g.(graph.Weighted))}
		}
		if !sort.IsSorted(paths) {
			t.Errorf("unexpected result for %q: got:%+v", test.name, paths)
		}
		if test.relaxed {
			continue
		}

		if len(gotIDs) != 0 {
			first := 0
			last := PathWeight(got[0], g.(graph.Weighted))
			for i := 1; i < len(got); i++ {
				w := PathWeight(got[i], g.(graph.Weighted))
				if w == last {
					continue
				}
				BySliceValues(gotIDs[first:i])
				first = i
				last = w
			}
			BySliceValues(gotIDs[first:])
		}

		var gotIDsNoLoop [][]int64
		for _, gotID := range gotIDs {
			gotIDModify := RemoveDuplicateElement(gotID)
			infoString = fmt.Sprintf("gotIDModify is:%+v", gotIDModify)
			nputil.TraceInfo(infoString)
			if len(gotID) == len(gotIDModify) {
				gotIDsNoLoop = append(gotIDsNoLoop, gotIDModify)
			}
		}
		infoString = fmt.Sprintf("PathsGroupNoLoop is:%+v", gotIDsNoLoop)
		nputil.TraceInfo(infoString)

		if !reflect.DeepEqual(test.wantPaths, gotIDsNoLoop) {
			t.Errorf("unexpected result for %q:\ngot: %v\nwant:%v", test.name, gotIDsNoLoop, test.wantPaths)
		}
	}
	infoString = fmt.Sprintf("=== RUN TestYenKSP END ===")
	nputil.TraceInfo(infoString)
}
