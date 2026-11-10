//go:build ignore

// Public Rack root-rejoin reproduction. Run: go run tools/probes/rack_rejoin.go.
package main

import (
	"fmt"
	"goframework.io/desktop-ui-rack"
)

func main() {
	nearest := rack.New(10)
	nearest.Add("main", rack.Box{X: 0, Y: 0, W: 100, H: 50})
	nearest.Add("earlier but farther", rack.Box{X: 1000, Y: 492, W: 100, H: 50})
	nearest.Add("later and flush", rack.Box{X: 1000, Y: 500, W: 100, H: 50})
	satellite := nearest.Add("satellite", rack.Box{X: 1500, Y: 800, W: 100, H: 50})
	nearest.MoveTo(satellite, 1000, 550)
	fmt.Printf("nearestAnchor=%d want=2 box=%+v\n", nearest.Pane(satellite).To, nearest.Pane(satellite).Box)
	for _, depth := range []int{2, 3, 4} {
		for _, gap := range []int{0, 2} {
			r := rack.New(10)
			r.Add("main", rack.Box{X: 0, Y: 0, W: 100, H: 20})
			for i := 1; i <= depth; i++ {
				r.Add(fmt.Sprint("pane", i), rack.Box{X: 500, Y: 200 + (i-1)*50, W: 100, H: 50})
				if i > 1 {
					r.Attach(i, i-1, rack.SideBottom)
				}
			}
			before := r.Boxes()
			anchor := r.Pane(depth).Box
			r.MoveTo(0, anchor.Right()+gap, anchor.Y+15)
			parents := make([]int, r.Len())
			stationary := true
			for i := 1; i < r.Len(); i++ {
				parents[i] = r.Pane(i).To
				stationary = stationary && r.Pane(i).Box == before[i]
			}
			parents[0] = r.Pane(0).To
			joined := r.Boxes()
			r.MoveTo(0, 900, 700)
			allFollow := true
			for i := 1; i < r.Len(); i++ {
				allFollow = allFollow && r.Pane(i).Box == joined[i].Moved(900-joined[0].X, 700-joined[0].Y)
			}
			fmt.Printf("depth=%d gap=%d parents=%v stationary=%t allFollow=%t main=%+v joined=%+v\n", depth, gap, parents, stationary, allFollow, joined[0], joined)
		}
	}
	branchedRejoin()
}

// Cover a Main that already carries a follower, and a side branch off the
// foreign path being reversed. Those are different from a bare linear chain.
func branchedRejoin() {
	r := rack.New(10)
	main := r.Add("main", rack.Box{W: 100, H: 20})
	follower := r.Add("main follower", rack.Box{W: 100, H: 20})
	r.Attach(follower, main, rack.SideBottom)
	root := r.Add("foreign root", rack.Box{X: 500, Y: 200, W: 100, H: 50})
	middle := r.Add("foreign middle", rack.Box{W: 100, H: 50})
	r.Attach(middle, root, rack.SideBottom)
	end := r.Add("foreign end", rack.Box{W: 100, H: 50})
	r.Attach(end, middle, rack.SideBottom)
	branch := r.Add("foreign branch", rack.Box{W: 100, H: 50})
	r.Attach(branch, middle, rack.SideLeft)
	before := r.Boxes()
	anchor := before[end]
	r.MoveTo(main, anchor.Right()+2, anchor.Y+15)
	joined := r.Boxes()
	stationary := true
	for _, i := range []int{root, middle, end, branch} {
		stationary = stationary && before[i] == joined[i]
	}
	parents := make([]int, r.Len())
	for i := range parents {
		parents[i] = r.Pane(i).To
	}
	followerAligned := joined[follower] == r.Pane(follower).Bond.Place(joined[main], joined[follower].W, joined[follower].H)
	r.MoveTo(main, joined[main].X+80, joined[main].Y+60)
	allFollow := true
	for i := range joined {
		allFollow = allFollow && r.Pane(i).Box == joined[i].Moved(80, 60)
	}
	fmt.Printf("branched component + existing Main follower: parents=%v stationary=%t followerAligned=%t allFollow=%t snappedMain=%+v\n", parents, stationary, followerAligned, allFollow, joined[main])
	if !stationary || !followerAligned || !allFollow || parents[main] != -1 || parents[follower] != main || parents[root] != middle || parents[middle] != end || parents[end] != main || parents[branch] != middle {
		panic("branched rejoin failed to preserve links and geometry")
	}
}
