package runnable

import "context"

func newManagerContainer(runnable Runnable) *managerContainer {
	ctx, cancel := context.WithCancel(context.Background())

	return &managerContainer{
		runnable: runnable,
		ctx:      ctx,
		shutdown: cancel,
	}
}

type managerContainer struct {
	runnable Runnable
	users    []*managerContainer

	ctx      context.Context
	shutdown func()

	err error
}

func (c *managerContainer) insertUser(container *managerContainer) {
	for _, item := range c.users {
		if item == container {
			return
		}
	}
	c.users = append(c.users, container)
}

func (c *managerContainer) name() string {
	return findName(c.runnable)
}

func (c *managerContainer) launch(completed chan *managerContainer, dying chan *managerContainer) {
	go func() {
		c.err = Recover(c.runnable).Run(c.ctx)
		completed <- c
		dying <- c
	}()
}

type managerContainerSet map[*managerContainer]bool

func newManagerContainerSet() managerContainerSet {
	return managerContainerSet{}
}

func (s managerContainerSet) insert(c *managerContainer) {
	s[c] = true
}

func (s managerContainerSet) contains(c *managerContainer) bool {
	_, has := s[c]
	return has
}

func (s managerContainerSet) containerHasRunningUsers(c *managerContainer) bool {
	for _, user := range c.users {
		if _, ok := s[user]; !ok {
			return true
		}
	}
	return false
}
