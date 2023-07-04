package runnable

func ExampleComponent() {
	ctx, cancel := initializeForExample()
	defer cancel()

	c := Component()
	c.Add(newDummyRunnable())

	c.Run(ctx)

	// Output:
	// component: starting services
	// component: starting processes
	// group: dummyRunnable started
	// dummyRunnable: started
	// component: context cancelled
	// component: shutting down processes
	// dummyRunnable: stopped
	// group: dummyRunnable stopped
	// component: shutting down services
	// component: shutdown complete
}
