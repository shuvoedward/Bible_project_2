package scheduler

import (
	"errors"
	"log/slog"
	"shuvoedward/Bible_project/internal/mailer"
	"sync"
	"time"
)

type Scheduler struct {
	NumWorkers   int
	TaskChannel  chan Task
	DelayedQueue *PriorityQueue
	DeadQueue    []Task
	Mailer       *mailer.Mailer
	Logger       *slog.Logger
	mu           *sync.Mutex
}

func NewScheduler(numWorkers int) *Scheduler {
	return &Scheduler{
		TaskChannel:  make(chan Task, 100),
		DelayedQueue: BuildMinHeap(),
		NumWorkers:   numWorkers,
		mu:           &sync.Mutex{},
	}
}

func (s Scheduler) Submit(task Task) {
	s.TaskChannel <- task
}

func (s Scheduler) Start() {
	for range s.NumWorkers {
		go s.worker(s.TaskChannel)
	}

	go s.processDelayedTasks()
}

func (s Scheduler) worker(taskChannel <-chan Task) {
	/*
		chan Task  // can send and receive
		<- chan Task  // can only receive (read-only)
		chan<- Task   // can only send (write-only)

		// About the for loop
		Keep receiving from this channel forever, and for each value recieved, do the loop body

		It's equivalent to writing:

		for {
			task, ok := <- taskChannel // receive from channel
			if !ok{
				break  	// channel closed, exit loop
			}
			processTask(task)
		}

	*/

	for task := range taskChannel {
		s.processTask(task)
	}
}

func (s Scheduler) processTask(task Task) {
	switch task.Type {
	case SendActivationEmail:
		s.sendActivationEmail(task)
	case SendPasswordResetEmail:
		s.sendPasswordResetEmail(task)
	case SendTokenActivatoinEmail:
		s.sendTokenActivatoinEmail(task)
	}
}

func (s Scheduler) sendActivationEmail(task Task) {
	data, ok := task.Data.(TaskEmailData)
	if !ok {
		panic("Scheduler process task error, wrong data type, has to be 'TaskEmailData")
	}

	if task.Retries < task.MaxRetries {
		return
	}

	err := s.Mailer.Send(data.Email, "user_welcome.tmpl", map[string]any{
		"username":      data.UserName,
		"activationURL": data.ActivationURL,
	})

	s.handleMailError(task, err)
}

func (s Scheduler) sendPasswordResetEmail(task Task) {
	data, ok := task.Data.(TaskPasswordResetEmail)
	if !ok {
		return
	}

	if task.Retries < task.MaxRetries {
		return
	}

	err := s.Mailer.Send(data.Email, "token_password_reset.tmpl", map[string]any{
		"passwordResetURL": data.PasswordResetURL,
	})

	s.handleMailError(task, err)
}

func (s Scheduler) sendTokenActivatoinEmail(task Task) {
	data, ok := task.Data.(TaskTokenActivationData)
	if !ok {
		return
	}

	if task.Retries < task.MaxRetries {
		return
	}
	err := s.Mailer.Send(data.Email, "token_activation.tmpl", map[string]any{
		"activationURL": data.ActivationURL,
	})

	s.handleMailError(task, err)
}

func (s Scheduler) handleMailError(task Task, err error) {
	var mailerErr *mailer.MailerError
	if errors.As(err, &mailerErr) {
		s.Logger.Error("email failed",
			"code", mailerErr.Code,
			"retriable", mailerErr.Retrieable,
			"metadata", mailerErr.Metadata,
		)

		if mailerErr.Retrieable {
			s.scheduleRetry(task)
		} else {
			s.DeadQueue = append(s.DeadQueue, task)
		}
	}
}

func (s Scheduler) scheduleRetry(task Task) {
	// compare time to calculate retry
	if task.Retries < task.MaxRetries {
		task.Retries++
		// time period: 2 minutes after executedat,  updated executed at
		switch task.Retries {
		case 1:
			task.ExecuteAt = task.CreatedAt.Add(2 * time.Minute)
		case 2:
			task.ExecuteAt = task.CreatedAt.Add(4 * time.Minute)
		case 3:
			task.ExecuteAt = task.CreatedAt.Add(8 * time.Minute)
		}

		s.DelayedQueue.Push(task)
	}
}

func (s Scheduler) processDelayedTasks() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()

		for s.DelayedQueue.Len() > 0 {
			task, ok := s.DelayedQueue.Peek().(Task)
			if !ok {
				// delete from the queue as it is wrong type
				break
			}

			if time.Now().Before(task.ExecuteAt) {
				break
			}

			s.DelayedQueue.Pop()
			s.Submit(task)
		}

		s.mu.Unlock()
	}
}
