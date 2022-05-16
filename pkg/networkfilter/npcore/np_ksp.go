package npcore

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/networkfilter/npksp"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	"math"
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

/***********************************************************************************************************************/
/*********************************************data structure*******************************************************************/
/***********************************************************************************************************************/
type DomainLinkKspGraph struct {
	graph func() graph.WeightedEdgeAdder
	edges []simple.WeightedEdge
}

type PathAttr struct {
	PathIds []int64
	Weight  float64
}

func NewDomainLinkKspGraph() *DomainLinkKspGraph {
	nputil.TraceInfoBegin("")

	var domainLinkKspGraph = new(DomainLinkKspGraph)

	nputil.TraceInfoEnd("")
	return domainLinkKspGraph
}

func BuildDomainLinkEgde() []simple.WeightedEdge {
	//var edges []simple.WeightedEdge
	var edges = []simple.WeightedEdge{
		{F: simple.Node(1), T: simple.Node(2), W: 1},
		{F: simple.Node(2), T: simple.Node(3), W: 2},
		{F: simple.Node(3), T: simple.Node(4), W: 3},
		{F: simple.Node(1), T: simple.Node(3), W: 1},
		{F: simple.Node(4), T: simple.Node(3), W: 3},
		{F: simple.Node(3), T: simple.Node(2), W: 2},
		{F: simple.Node(2), T: simple.Node(1), W: 1},
		{F: simple.Node(4), T: simple.Node(2), W: 2},
	}
	/*var edges = []simple.WeightedEdge([]simple.WeightedEdge{
		{F: simple.Node('A'), T: simple.Node('B'), W: 1},
		{F: simple.Node('B'), T: simple.Node('C'), W: 2},
		{F: simple.Node('C'), T: simple.Node('D'), W: 3},
		{F: simple.Node('A'), T: simple.Node('C'), W: 1},
		{F: simple.Node('D'), T: simple.Node('C'), W: 3},
		{F: simple.Node('C'), T: simple.Node('B'), W: 2},
		{F: simple.Node('B'), T: simple.Node('A'), W: 1},
		{F: simple.Node('D'), T: simple.Node('B'), W: 2},
	})*/
	/*var edges = []simple.WeightedEdge{}*/
	/*var edges = []simple.WeightedEdge{
		{F: simple.Node(0), T: simple.Node(1), W: 3},
		{F: simple.Node(0), T: simple.Node(2), W: 3},
		{F: simple.Node(0), T: simple.Node(3), W: 3},
	}*/
	/*var edges = []simple.WeightedEdge{
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
	    }*/
	fmt.Printf("BuildDomainLinkEgde are:%+v\n", edges)
	return edges
}

func GetPathList(paths [][]graph.Node, g graph.Weighted) []PathAttr {
	nputil.TraceInfoBegin("")

	var pathAttrList []PathAttr
	if paths == nil {
		return nil
	}
	pathAttrList = make([]PathAttr, len(paths))

	//ids := make([][]int64, len(paths))
	for i, p := range paths {
		if p == nil {
			continue
		}
		pathAttrList[i].PathIds = make([]int64, len(p))
		for j, n := range p {
			pathAttrList[i].PathIds[j] = n.ID()
			pathAttrList[i].Weight = npksp.PathWeight(p, g)
		}
	}
	nputil.TraceInfoEnd("")
	return pathAttrList
}

func SortPathValues(pathAttr []PathAttr) {
	nputil.TraceInfoBegin("")
	sort.Slice(pathAttr, func(i, j int) bool {
		a := pathAttr[i].PathIds
		b := pathAttr[j].PathIds
		l := len(a)
		if len(b) < l {
			l = len(b)
		}
		for k, v := range a[:l] {
			if v < b[k] {
				return true
			}
			if v > b[k] {
				return false
			}
		}
		return len(a) < len(b)
	})
	nputil.TraceInfoEnd("")
}

func BuildDomainLinkGraph() {
	nputil.TraceInfoBegin("")

	var domainLinkKspGraph = NewDomainLinkKspGraph()
	domainLinkKspGraph.graph = func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) }

	edges := BuildDomainLinkEgde()
	domainLinkKspGraph.edges = edges
	fmt.Printf("domainLinkKspGraph  is:%+v\n", domainLinkKspGraph)

	query := simple.Edge{F: simple.Node(1), T: simple.Node(4)}
	paths := KspCalcDomainPath(domainLinkKspGraph, 10, query)
	fmt.Printf("networkPlugin paths are:%+v\n", paths)

	nputil.TraceInfoEnd("")
}

func KspCalcDomainPath(domainLinkGraph *DomainLinkKspGraph, spfCalcMaxNum int, query simple.Edge) []PathAttr {
	nputil.TraceInfoBegin("")

	g := domainLinkGraph.graph()
	for _, e := range domainLinkGraph.edges {
		fmt.Printf(" domainLinkGraph.edge is (%+v)\n", e)
		g.SetWeightedEdge(e)
	}
	got := npksp.YenKShortestPaths(g.(graph.Graph), spfCalcMaxNum, query.From(), query.To())
	fmt.Printf("got origin is:%+v\n", got)
	PathList := GetPathList(got, g.(graph.Weighted))
	fmt.Printf("PathList origin is:%+v\n", PathList)

	paths := make(npksp.ByPathWeight, len(PathList))
	for i, p := range got {
		paths[i] = npksp.YenShortest{Path: p, Weight: npksp.PathWeight(p, g.(graph.Weighted))}
		fmt.Printf("paths[%d] is:%+v\n", i, paths[i])
	}
	if !sort.IsSorted(paths) {
		fmt.Printf("unexpected result got:%+v\n", paths)
	}
	if len(PathList) != 0 {
		first := 0
		last := npksp.PathWeight(got[0], g.(graph.Weighted))
		for i := 1; i < len(got); i++ {
			w := npksp.PathWeight(got[i], g.(graph.Weighted))
			if w == last {
				continue
			}
			SortPathValues(PathList[first:i])
			first = i
			last = w
		}
		SortPathValues(PathList[first:])
	}
	fmt.Printf("PathList modify is:%+v\n", PathList)

	var PathsGroupNoLoop []PathAttr
	for _, Path := range PathList {
		fmt.Printf("Path is:%+v\n", Path)
		gotIDModify := npksp.RemoveDuplicateElement(Path.PathIds)
		fmt.Printf("gotIDModify is:%+v\n", gotIDModify)
		if len(Path.PathIds) == len(gotIDModify) {
			PathsGroupNoLoop = append(PathsGroupNoLoop, Path)
		}
	}
	//Path为domainspfid list
	fmt.Printf("PathsGroupNoLoop is:%+v\n", PathsGroupNoLoop)

	nputil.TraceInfoEnd("")
	return PathsGroupNoLoop
}
