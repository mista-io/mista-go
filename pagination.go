package mista

import (
	"context"
	"encoding/json"
)

// PageMeta describes where a page sits in the full result set.
type PageMeta struct {
	CurrentPage int
	LastPage    int
	PerPage     int
	Total       int
}

// Page is one page of results.
type Page[T any] struct {
	Items []T
	Meta  PageMeta
}

// HasNextPage reports whether more pages follow.
func (p *Page[T]) HasNextPage() bool { return p.Meta.CurrentPage < p.Meta.LastPage }

// pageJSON reads both the Laravel paginator ({current_page, data, ...}) and the
// Voice shape ({items, pagination: {...}}).
type pageJSON[T any] struct {
	Data        []T      `json:"data"`
	Items       []T      `json:"items"`
	CurrentPage *FlexInt `json:"current_page"`
	LastPage    *FlexInt `json:"last_page"`
	PerPage     *FlexInt `json:"per_page"`
	Total       *FlexInt `json:"total"`
	Pagination  *struct {
		CurrentPage *FlexInt `json:"current_page"`
		LastPage    *FlexInt `json:"last_page"`
		PerPage     *FlexInt `json:"per_page"`
		Total       *FlexInt `json:"total"`
	} `json:"pagination"`
}

func (p *Page[T]) UnmarshalJSON(b []byte) error {
	var raw pageJSON[T]
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	items := raw.Data
	current, last, per, total := raw.CurrentPage, raw.LastPage, raw.PerPage, raw.Total
	if raw.Pagination != nil {
		items = raw.Items
		current, last, per, total = raw.Pagination.CurrentPage, raw.Pagination.LastPage, raw.Pagination.PerPage, raw.Pagination.Total
	}
	if items == nil {
		items = []T{}
	}
	p.Items = items
	p.Meta = PageMeta{
		CurrentPage: intOr(current, 1),
		LastPage:    intOr(last, 1),
		PerPage:     intOr(per, len(items)),
		Total:       intOr(total, len(items)),
	}
	return nil
}

func intOr(v *FlexInt, fallback int) int {
	if v == nil {
		return fallback
	}
	return int(*v)
}

func emptyPage[T any]() *Page[T] {
	return &Page[T]{Items: []T{}, Meta: PageMeta{CurrentPage: 1, LastPage: 1}}
}

// Iter walks every item across all pages, fetching pages as needed:
//
//	it := client.Logs.ListAutoPaging(ctx, &mista.ListMessagesParams{Status: "Delivered"})
//	for it.Next(ctx) {
//		msg := it.Current()
//	}
//	if err := it.Err(); err != nil { ... }
type Iter[T any] struct {
	fetch   func(ctx context.Context, page int) (*Page[T], error)
	page    *Page[T]
	next    int
	index   int
	current T
	err     error
}

func newIter[T any](start int, fetch func(ctx context.Context, page int) (*Page[T], error)) *Iter[T] {
	if start < 1 {
		start = 1
	}
	return &Iter[T]{fetch: fetch, next: start}
}

// Next advances to the next item, fetching the next page when needed. It
// returns false when there are no more items or an error occurred.
func (it *Iter[T]) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	for it.page == nil || it.index >= len(it.page.Items) {
		if it.page != nil && !it.page.HasNextPage() {
			return false
		}
		page, err := it.fetch(ctx, it.next)
		if err != nil {
			it.err = err
			return false
		}
		it.page, it.index = page, 0
		it.next = page.Meta.CurrentPage + 1
		if len(page.Items) == 0 && !page.HasNextPage() {
			return false
		}
	}
	it.current = it.page.Items[it.index]
	it.index++
	return true
}

// Current returns the item Next moved to.
func (it *Iter[T]) Current() T { return it.current }

// Err returns the error that stopped iteration, if any.
func (it *Iter[T]) Err() error { return it.err }
