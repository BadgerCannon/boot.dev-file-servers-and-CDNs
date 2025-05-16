package main

import (
	"log"
	"math"
)

type Ratio struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (r Ratio) Orientation() string {
	if r.Height > r.Width {
		return "portrait"
	} else if r.Width > r.Height {
		return "landscape"
	} else {
		return "other"
	}
}

func (r *Ratio) Reduce() {
	factor := gcf(r.Height, r.Width)
	log.Printf("GCF: %d", factor)

	r.Height = r.Height / factor
	r.Width = r.Width / factor
}

func gcf(x, y int) int {
	var a, b int
	factor := math.MaxInt

	if x > y {
		a = x
		b = y
	} else {
		a = y
		b = x
	}

	factor = a - b
	if factor == 0 {
		return b
	}
	return gcf(b, factor)
}
