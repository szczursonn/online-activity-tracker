package viewer

import (
	"iter"
)

var cardGridTemplate = mustParseTemplate("card_grid.html")

type cardGridPageData struct {
	Title string
	Items iter.Seq[cardGridPageDataItem]
}

type cardGridPageDataItem struct {
	Href     string
	Name     string
	ImageURL string
	ImageAlt string
}

func mapSeq[T any, U any](s []T, f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for _, v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}
