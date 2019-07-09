package orchestrator

// Package orchestrator is an algorithm that manages the work of a cluster of
// nodes. It ensures each piece of work has a worker assigned to it.
//
// The Orchestrator stores a set of expected tasks. Each term, it reaches out
// to the cluster to gather what each node is working on. These tasks are
// called the actual tasks. The Orchestrator adjusts the nodes workload to
// attempt to match the expected tasks.
//
// The expected workload is stored in memory. Therefore, if the process is
// restarted the task list is lost. A system with persistence is required to
// ensure the workload is not lost (e.g., database).

import (
	"context"
	"io/ioutil"
	"log"
	"sync"
	"time"
)

// Orchestrator stores the expected workload and reaches out to the cluster
// to see what the actual workload is. It then tries to fix the delta.
//
// The expected task list can be altered via AddTask, RemoveTask and
// UpdateTasks. Each method is safe to be called on multiple go-routines.
type Orchestrator struct {
	log     Logger
	c       Communicator
	s       func(TermStats)
	timeout time.Duration

	mu            sync.Mutex
	workers       []interface{}
	expectedTasks []Task

	// LastActual is set each term. It is only used for a user who wants to
	// know the state of the worker cluster from the last term.
	lastActual []WorkerState
}

// New creates a new Orchestrator.
func New(c Communicator, opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		c:       c,
		s:       func(TermStats) {},
		log:     log.New(ioutil.Discard, "", 0),
		timeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

//OrchestratorOption configures an Orchestrator.
type OrchestratorOption func(*Orchestrator)

// Logger is used to write information.
type Logger interface {
	// Print calls l.Output to print to the logger. Arguments are handled in
	// the manner of fmt.Print.
	Printf(format string, v ...interface{})
}

// WithLogger sets the logger for the Orchestrator. Defaults to silent logger.
func WithLogger(l Logger) OrchestratorOption {
	return func(o *Orchestrator) {
		o.log = l
	}
}

// TermStats is the information about the last processed term. It is passed
// to a stats handler. See WithStats().
type TermStats struct {
	// WorkerCount is the number of workers that responded without an error
	// to a List request.
	WorkerCount int
}

// WithStats sets the stats handler for the Orchestrator. The stats handler
// is invoked for each term, with what the Orchestrator wrote to the
// Communicator.
func WithStats(f func(TermStats)) OrchestratorOption {
	return func(o *Orchestrator) {
		o.s = f
	}
}

// WithCommunicatorTimeout sets the timeout for the communication to respond.
// Defaults to 10 seconds.
func WithCommunicatorTimeout(t time.Duration) OrchestratorOption {
	return func(o *Orchestrator) {
		o.timeout = t
	}
}

// Communicator manages the intra communication between the Orchestrator and
// the node cluster. Each method must be safe to call on many go-routines.
// The given context represents the state of the term. Therefore, the
// Communicator is expected to cancel immediately if the context is done.
type Communicator interface {
	// List returns the workload from the given worker.
	List(ctx context.Context, worker interface{}) ([]interface{}, error)

	// Add adds the given task to the worker. The error only logged (for now).
	// It is assumed that if the worker returns an error trying to update, the
	// next term will fix the problem and move the task elsewhere.
	Add(ctx context.Context, worker, task interface{}) error

	// Removes the given task from the worker. The error is only logged (for
	// now). It is assumed that if the worker is returning an error, then it
	// is either not doing the task because the worker is down, or there is a
	// network partition and a future term will fix the problem.
	Remove(ctx context.Context, worker, task interface{}) error
}

// NextTerm reaches out to the cluster to gather to actual workload. It then
// attempts to fix the delta between actual and expected. The lifecycle of
// the term is managed by the given context.
func (o *Orchestrator) NextTerm(ctx context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Gather the state of the world from the workers.
	var actual map[interface{}][]interface{}
	actual, o.lastActual = o.collectActual(ctx)
	toAdd, toRemove := o.delta(actual)

	// Rebalance tasks among workers.
	toAdd, toRemove = o.Rebalance(toAdd, toRemove, actual)
	counts := o.counts(actual, toRemove)

	for worker, tasks := range toRemove {
		for _, task := range tasks {
			// Remove the task from the workers.
			removeCtx, _ := context.WithTimeout(ctx, o.timeout)
			o.c.Remove(removeCtx, worker, task)
		}
	}

	for task, missing := range toAdd {
		history := make(map[interface{}]bool)
		for i := 0; i < missing; i++ {
			counts = o.assignTask(ctx,
				task,
				counts,
				actual,
				history,
			)
		}
	}

	o.s(TermStats{
		WorkerCount: len(actual),
	})
}

// rebalance will rebalance tasks across the workers. If any worker has too
// many tasks, it will be added to the remove map, and added to the returned
// add slice.
func (o *Orchestrator) Rebalance(
	toAdd map[interface{}]int,
	toRemove,
	actual map[interface{}][]interface{},
) (map[interface{}]int, map[interface{}][]interface{}) {

	counts := o.counts(actual, toRemove)
	if len(counts) == 0 {
		return toAdd, toRemove
	}

	var total int
	for _, c := range counts {
		total += c.count
	}

	for _, addCount := range toAdd {
		total += addCount
	}

	maxPerNode := total / len(counts)
	if maxPerNode == 0 || total%len(counts) != 0 {
		maxPerNode++
	}

	for _, c := range counts {
		if c.count > maxPerNode && len(actual[c.name]) > 0 {
			task := actual[c.name][0]
			toRemove[c.name] = append(toRemove[c.name], task)
			toAdd[task]++
		}
	}

	return toAdd, toRemove
}

// assignTask tries to find a worker that does not have too many tasks
// assigned. If it encounters a worker with too many tasks, it will remove
// it from the pool and try again.
func (o *Orchestrator) assignTask(
	ctx context.Context,
	task interface{},
	counts []countInfo,
	actual map[interface{}][]interface{},
	history map[interface{}]bool,
) []countInfo {

	for i, info := range counts {
		// Ensure that each worker gets an even amount of work assigned.
		// Therefore if a worker gets its fair share, remove it from the worker
		// pool for this term. This also accounts for there being a non-divisbile
		// amount of tasks per workers.
		info.count++
		activeWorkers := len(actual)
		totalTasks := o.totalTaskCount()
		if info.count > totalTasks/activeWorkers+totalTasks%activeWorkers {
			counts = append(counts[:i], counts[i+1:]...)

			// Return true saying the worker pool was adjusted and the task was
			// not assigned.
			return o.assignTask(ctx, task, counts, actual, history)
		}

		// Ensure we haven't assigned this task to the worker already.
		if history[info.name] || o.contains(task, actual[info.name]) >= 0 {
			continue
		}
		history[info.name] = true

		// Update the count for the worker.
		counts[i] = countInfo{
			name:  info.name,
			count: info.count,
		}

		// Assign the task to the worker.
		o.log.Printf("Adding task %s to %s.", task, info.name)
		addCtx, _ := context.WithTimeout(ctx, o.timeout)
		o.c.Add(addCtx, info.name, task)

		// Move adjusted count to end of slice to help with fairness
		c := counts[i]
		counts = append(append(counts[:i], counts[i+1:]...), c)

		break
	}

	return counts
}

// totalTaskCount calculates the total number of expected task instances.
func (o *Orchestrator) totalTaskCount() int {
	var total int
	for _, t := range o.expectedTasks {
		total += t.Instances
	}
	return total
}

// countInfo stores information used to assign tasks to workers.
type countInfo struct {
	name  interface{}
	count int
}

// counts looks at each worker and gathers the number of tasks each has.
func (o *Orchestrator) counts(actual, toRemove map[interface{}][]interface{}) []countInfo {
	var results []countInfo
	for k, v := range actual {
		if len(v) == 0 {
			results = append(results, countInfo{
				name:  k,
				count: 0,
			})
			continue
		}
		results = append(results, countInfo{
			name:  k,
			count: len(v) - len(toRemove[k]),
		})
	}
	return results
}

// collectActual reaches out to each worker and gets their state of the world.
// Each worker is queried in parallel. If a worker returns an error while
// trying to list the tasks, it will be logged and not considered for what
// workers should be assigned work.
func (o *Orchestrator) collectActual(ctx context.Context) (map[interface{}][]interface{}, []WorkerState) {
	type result struct {
		name   interface{}
		actual []interface{}
		err    error
	}

	listCtx, _ := context.WithTimeout(ctx, o.timeout)
	results := make(chan result, len(o.workers))
	errs := make(chan result, len(o.workers))
	for _, worker := range o.workers {
		go func(worker interface{}) {
			r, err := o.c.List(listCtx, worker)
			if err != nil {
				errs <- result{name: worker, err: err}
				return
			}

			results <- result{name: worker, actual: r}
		}(worker)
	}

	t := time.NewTimer(o.timeout)
	var state []WorkerState
	actual := make(map[interface{}][]interface{})
	for i := 0; i < len(o.workers); i++ {
		select {
		case <-ctx.Done():
			break
		case r := <-results:
			actual[r.name] = r.actual
			state = append(state, WorkerState{Name: r.name, Tasks: r.actual})
		case err := <-errs:
			o.log.Printf("Error trying to list tasks from %s: %s", err.name, err.err)
		case <-t.C:
			o.log.Printf("Communicator timeout. Using results available...")
			break
		}
	}

	return actual, state
}

// delta finds what should be added and removed to make actual match the
// expected.
func (o *Orchestrator) delta(actual map[interface{}][]interface{}) (toAdd map[interface{}]int, toRemove map[interface{}][]interface{}) {
	toRemove = make(map[interface{}][]interface{})
	toAdd = make(map[interface{}]int)
	expectedTasks := make([]Task, len(o.expectedTasks))
	copy(expectedTasks, o.expectedTasks)

	for _, task := range o.expectedTasks {
		needs := o.hasEnough(task, actual)
		if needs == 0 {
			continue
		}
		toAdd[task.Name] = needs
	}

	for worker, tasks := range actual {
		for _, task := range tasks {
			if idx := o.containsTask(task, expectedTasks); idx >= 0 {
				expectedTasks[idx].Instances--
				if expectedTasks[idx].Instances == 0 {
					expectedTasks = append(expectedTasks[0:idx], expectedTasks[idx+1:]...)
				}
				continue
			}
			toRemove[worker] = append(toRemove[worker], task)
		}
	}

	return toAdd, toRemove
}

// hasEnough looks at each task in the given actual list and ensures
// a worker node is servicing the task.
func (o *Orchestrator) hasEnough(t Task, actual map[interface{}][]interface{}) (needs int) {
	var count int
	for _, a := range actual {
		if o.contains(t.Name, a) >= 0 {
			count++
		}
	}

	return t.Instances - count
}

// contains returns the index of the given interface{} (x) in the slice y. If the
// interface{} is not present in the slice, it returns -1.
func (o *Orchestrator) contains(x interface{}, y []interface{}) int {
	for i, t := range y {
		if t == x {
			return i
		}
	}

	return -1
}

// containsTask returns the index of the given task name in the tasks. If the
// task is not found, it returns -1.
func (o *Orchestrator) containsTask(task interface{}, tasks []Task) int {
	for i, t := range tasks {
		if t.Name == task {
			return i
		}
	}

	return -1
}

// AddWorker adds a worker to the known worker cluster. The update will not
// take affect until the next term. It is safe to invoke AddWorker,
// RemoveWorkers and UpdateWorkers on multiple go-routines.
func (o *Orchestrator) AddWorker(worker interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure we don't alreay have this worker
	for _, w := range o.workers {
		if w == worker {
			return
		}
	}

	o.workers = append(o.workers, worker)
}

// RemoveWorker removes a worker from the known worker cluster. The update
// will not take affect until the next term. It is safe to invoke AddWorker,
// RemoveWorkers and UpdateWorkers on multiple go-routines.
func (o *Orchestrator) RemoveWorker(worker interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()

	idx := o.contains(worker, o.workers)
	if idx < 0 {
		return
	}

	o.workers = append(o.workers[:idx], o.workers[idx+1:]...)
}

// UpdateWorkers overwrites the expected worker list. The update will not take
// affect until the next term. It is safe to invoke AddWorker, RemoveWorker
// and UpdateWorkers on multiple go-routines.
func (o *Orchestrator) UpdateWorkers(workers []interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.workers = workers
}

// Task stores the required information for a task.
type Task struct {
	Name      interface{}
	Instances int
}

// AddTask adds a new task to the expected workload. The update will not take
// affect until the next term. It is safe to invoke AddTask, RemoveTask and
// UpdateTasks on multiple go-routines.
func (o *Orchestrator) AddTask(task interface{}, opts ...TaskOption) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure we don't already have this task
	for _, t := range o.expectedTasks {
		if task == t.Name {
			return
		}
	}

	t := Task{Name: task, Instances: 1}
	for _, opt := range opts {
		opt(&t)
	}

	o.expectedTasks = append(o.expectedTasks, t)
}

// TaskOption is used to configure a task when it is being added.
type TaskOption func(*Task)

// WithTaskInstances configures the number of tasks. Defaults to 1.
func WithTaskInstances(i int) TaskOption {
	return func(t *Task) {
		t.Instances = i
	}
}

// RemoveTask removes a task from the expected workload. The update will not
// take affect until the next term. It is safe to invoke AddTask, RemoveTask
// and UpdateTasks on multiple go-routines.
func (o *Orchestrator) RemoveTask(task interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()

	idx := o.containsTask(task, o.expectedTasks)
	if idx < 0 {
		return
	}

	o.expectedTasks = append(o.expectedTasks[:idx], o.expectedTasks[idx+1:]...)
}

// UpdateTasks overwrites the expected task list. The update will not take
// affect until the next term. It is safe to invoke AddTask, RemoveTask and
// UpdateTasks on multiple go-routines.
func (o *Orchestrator) UpdateTasks(tasks []Task) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.expectedTasks = tasks
}

// ListExpectedTasks returns the curent list of the expected tasks.
func (o *Orchestrator) ListExpectedTasks() []Task {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.expectedTasks
}

// WorkerState stores the state of a worker.
type WorkerState struct {
	// Name is the given name of a worker.
	Name interface{}

	// Tasks is the task names the worker is servicing.
	Tasks []interface{}
}

// LastActual returns the actual from the last term. It will return nil
// before the first term.
func (o *Orchestrator) LastActual() []WorkerState {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.lastActual
}
