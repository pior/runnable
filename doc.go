// Package runnable manages the lifecycle of long-running processes.
//
// Everything is a [Runnable]: a Run method that blocks until its context is
// cancelled or its work is done. Shutdown is driven by context cancellation.
//
// [Manager] runs several runnables together and stops them in order: processes
// first, then the services they depend on. The entry points [Run], [RunFunc]
// and [RunGroup] add signal handling for main.
//
// Wrappers compose behavior around a runnable: [HTTPServer], [Restart],
// [Schedule], [Recover], [Signal], [Closer], [Func] and [Named].
//
// Names flow through the context. A parent [Manager] or [Named] assigns them,
// wrappers pass them through, and [NameFromContext] reads them.
package runnable
