package runnable

type AppOption func(*component)

// AppOrder defines the order in which runnables are started and stopped.
// Runnables with the lowest value are started first and stopped last.
func AppOrder(order int) AppOption {
	return func(c *component) { c.order = order }
}

// AppOrderFirst sets the order to 1
func AppOrderFirst(c *component) {
	c.order = 1
}

// AppOrderDB sets the order to 1
func AppOrderDB(c *component) {
	c.order = 3
}

// AppOrderService sets the order to 5
func AppOrderService(c *component) {
	c.order = 5
}

// AppOrderDefault sets the order to 7
func AppOrderDefault(c *component) {
	c.order = 7
}

// AppOrderLast sets the order to 9
func AppOrderLast(c *component) {
	c.order = 9
}

func applyAppOptions(c *component, opts []AppOption) {
	AppOrderDefault(c)
	for _, opt := range opts {
		opt(c)
	}
}
