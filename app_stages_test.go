package runnable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_prepareStages(t *testing.T) {
	ctx := context.Background()

	r1, r2 := newDummyRunnable(), newDummyRunnable()

	stages := prepareStages(ctx, []*component{
		{order: 1, runnable: r1},
		{order: 2, runnable: r2},
	})

	require.Equal(t, 1, stages.fromLowToHighOrders()[0].order)
	require.Equal(t, 2, stages.fromLowToHighOrders()[1].order)

	require.Equal(t, 2, stages.fromHighToLowOrders()[0].order)
	require.Equal(t, 1, stages.fromHighToLowOrders()[1].order)
}
