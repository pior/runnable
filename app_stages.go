package runnable

import (
	"context"
	"sort"
)

type stage struct {
	order      int
	components []*component

	ctx    context.Context
	cancel func()
}

type stages map[int]*stage

func prepareStages(ctx context.Context, components []*component) stages {
	s := stages{}
	for _, c := range components {
		if _, ok := s[c.order]; !ok {
			s[c.order] = &stage{}
		}
		st := s[c.order]
		st.order = c.order

		st.components = append(st.components, c)
		st.ctx, st.cancel = context.WithCancel(ctx)
	}
	return s
}

func (s stages) list() []*stage {
	values := []*stage{}
	for _, v := range s {
		values = append(values, v)
	}
	return values
}

func (s stages) fromLowToHighOrders() []*stage {
	values := s.list()
	sort.Slice(values, func(i, j int) bool { return values[i].order < values[j].order })
	return values
}

func (s stages) fromHighToLowOrders() []*stage {
	values := s.list()
	sort.Slice(values, func(i, j int) bool { return values[i].order >= values[j].order })
	return values
}

func (s stage) componentsRunning() []*component {
	var components []*component
	for _, c := range s.components {
		if !c.stopped.isSet() {
			components = append(components, c)
		}
	}
	return components
}
