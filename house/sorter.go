package house

import (
  "sort"
  "github.com/MobRulesGames/GoLLRB/llrb"
  "github.com/MobRulesGames/glop/util/algorithm"
  "github.com/MobRulesGames/haunts/base"
)

type endpoint struct {
  RectObject
  first bool
}

func firstPoint(r RectObject) (int, int) {
  x, y := r.Pos()
  _, dy := r.Dims()
  return x, y + dy
}
func lastPoint(r RectObject) (int, int) {
  x, y := r.Pos()
  dx, _ := r.Dims()
  return x + dx, y
}
func firstAndLastPoints(r RectObject) (x1, y1, x2, y2 int) {
  x, y := r.Pos()
  dx, dy := r.Dims()
  return x, y + dy, x + dx, y
}

type endpointArray []endpoint

func (e endpointArray) Len() int {
  return len(e)
}
func (e endpointArray) Less(i, j int) bool {
  var ix, iy, jx, jy int
  if e[i].first {
    ix, iy = firstPoint(e[i])
  } else {
    ix, iy = lastPoint(e[i])
  }
  if e[j].first {
    jx, jy = firstPoint(e[j])
  } else {
    jx, jy = lastPoint(e[j])
  }
  if ix-iy == jx-jy {
    return e[j].first
  }
  return ix-iy < jx-jy
}
func (e endpointArray) Swap(i, j int) {
  e[i], e[j] = e[j], e[i]
}

func dist(x, y int) int {
  return x*x + y*y
}
func width(dx, dy int) int {
  return dx + dy
}
func pos(x, y int) int {
  return x - y
}

type adag [][]int

func (a adag) NumVertex() int {
  return len(a)
}
func (a adag) Successors(n int) []int {
  return a[n]
}

func OrderRectObjects(ra []RectObject) []RectObject {
  p := order(ra)
  if p == nil {
    return ra
  }
  r := make([]RectObject, len(ra))
  for i := range p {
    r[i] = ra[p[i]]
  }
  return r
}

func order(input []RectObject) []int {
  defer func() {
    if err := recover(); err != nil {
      base.Error().Printf("Failure in sorting: %v", err)
    }
  }()
  var minx, miny int
  for _, r := range input {
    x, y := r.Pos()
    if x < minx {
      minx = x
    }
    if y < miny {
      miny = y
    }
  }

  ra := make([]RectObject, len(input))
  for i, r := range input {
    x, y := r.Pos()
    dx, dy := r.Dims()
    ra[i] = arog{x - minx + 1, y - miny + 1, dx, dy}
  }

  mapping := make(map[RectObject]int, len(ra))
  for i := range ra {
    mapping[ra[i]] = i
  }
  var e endpointArray
  for i := range ra {
    e = append(e, endpoint{RectObject: ra[i], first: false})
    e = append(e, endpoint{RectObject: ra[i], first: true})
  }
  sort.Sort(e)
  var sweep_pos int
  less_func := func(_a, _b interface{}) bool {
    a := _a.(RectObject)
    b := _b.(RectObject)
    ax, ay, ax2, ay2 := firstAndLastPoints(a)
    da := ax*ax + ay*ay
    da2 := ax2*ax2 + ay2*ay2
    w_a := width(a.Dims())
    bx, by, bx2, by2 := firstAndLastPoints(b)
    db := bx*bx + by*by
    db2 := bx2*bx2 + by2*by2
    w_b := width(b.Dims())
    va := w_b * (w_a*da + (da2-da)*(sweep_pos-pos(ax, ay)))
    vb := w_a * (w_b*db + (db2-db)*(sweep_pos-pos(bx, by)))
    return va < vb
  }
  l := llrb.New(less_func)

  dag := make(adag, len(ra))

  for _, p := range e {
    if p.first {
      sweep_pos = pos(firstPoint(p.RectObject))
      l.ReplaceOrInsert(p.RectObject)
      lower := l.LowerBound(p.RectObject)
      upper := l.UpperBound(p.RectObject)
      if lower != nil {
        index := mapping[lower.(RectObject)]
        dag[index] = append(dag[index], mapping[p.RectObject])
      }
      if upper != nil {
        index := mapping[p.RectObject]
        dag[index] = append(dag[index], mapping[upper.(RectObject)])
      }
    } else {
      l.Delete(p.RectObject)
    }
  }
  return algorithm.TopoSort(dag)
}

type arog struct {
  x, y, dx, dy int
}

func (a arog) Pos() (int, int)  { return a.x, a.y }
func (a arog) Dims() (int, int) { return a.dx, a.dy }
