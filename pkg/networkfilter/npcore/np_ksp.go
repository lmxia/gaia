package npcore

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/networkfilter/npksp"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
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

func KspCalcDomainPath(domainLinkGraph *DomainLinkKspGraph, spfCalcMaxNum int, query simple.Edge) []PathAttr {
	nputil.TraceInfoBegin("")

	g := domainLinkGraph.graph()
	for _, e := range domainLinkGraph.edges {
		infoString := fmt.Sprintf(" domainLinkGraph.edge is (%+v)", e)
		nputil.TraceInfo(infoString)
		g.SetWeightedEdge(e)
	}
	got := npksp.YenKShortestPaths(g.(graph.Graph), spfCalcMaxNum, query.From(), query.To())
	infoString := fmt.Sprintf("got origin is:%+v", got)
	nputil.TraceInfo(infoString)
	PathList := GetPathList(got, g.(graph.Weighted))
	infoString = fmt.Sprintf("PathList origin is:%+v", PathList)
	nputil.TraceInfo(infoString)

	paths := make(npksp.ByPathWeight, len(PathList))
	for i, p := range got {
		paths[i] = npksp.YenShortest{Path: p, Weight: npksp.PathWeight(p, g.(graph.Weighted))}
	}
	if !sort.IsSorted(paths) {
		infoString = fmt.Sprintf("unexpected result got:%+v", paths)
		nputil.TraceInfo(infoString)
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
	infoString = fmt.Sprintf("PathList modify is:%+v", PathList)
	nputil.TraceInfo(infoString)

	var PathsGroupNoLoop []PathAttr
	for _, Path := range PathList {
		gotIDModify := npksp.RemoveDuplicateElement(Path.PathIds)
		infoString = fmt.Sprintf("gotIDModify is:%+v", gotIDModify)
		nputil.TraceInfo(infoString)
		if len(Path.PathIds) == len(gotIDModify) {
			PathsGroupNoLoop = append(PathsGroupNoLoop, Path)
		}
	}
	//Path为domainspfid list
	infoString = fmt.Sprintf("PathsGroupNoLoop is:%+v", PathsGroupNoLoop)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return PathsGroupNoLoop
}
