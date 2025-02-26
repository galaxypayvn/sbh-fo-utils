package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"code.finan.one/finan-one-be/fo-utils/cronlog"
	"code.finan.one/finan-one-be/fo-utils/l"
	wokerstatus "code.finan.one/finan-one-be/fo-utils/worker/status"
)

// Scheduler ...
type Scheduler struct {
	ScheduleJobs []*SchedulerGroup
	ctx          context.Context
	lock         sync.Mutex
	c            *cron.Cron
	ll           l.Logger
}

// NewScheduler ...
func NewScheduler(ctx context.Context, sj []*SchedulerGroup, ll l.Logger) *Scheduler {
	cl := cronlog.NewCronLogger(ll)

	return &Scheduler{
		ScheduleJobs: sj,
		ctx:          ctx,
		lock:         sync.Mutex{},
		c: cron.New(
			cron.WithParser(
				cron.NewParser(
					cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)),
			cron.WithChain(cron.SkipIfStillRunning(cl)),
		),
		ll: ll,
	}
}

// Close ...
func (s *Scheduler) Close() {
	s.ll.Info("Scheduler closed")
}

// Start ...
func (s *Scheduler) Start() {
	if len(s.ScheduleJobs) == 0 {
		return
	}
	s.ll.Info("Start scheduler")
	// add job to schedule
	for _, item := range s.ScheduleJobs {
		jobCfg := item
		st := jobCfg.pl
		id, err := s.c.AddFunc(jobCfg.spec, func() {
			st.Before()
			s.ll.S.Infof("[%v] Running job ", st.GetNameWithSuffix())
			localMaxTry := jobCfg.retries

			if localMaxTry == 0 {
				localMaxTry = 1
			}

			if localMaxTry < 0 || localMaxTry > AllowedMaxRetries {
				localMaxTry = DefaultRetries
			}
			var retries int
			var shouldRetry = true

			for retries < localMaxTry && shouldRetry {
				retries++
				resp := st.Handle()
				code := wokerstatus.Status(resp.Code)
				switch code {
				case wokerstatus.Success:
					s.ll.S.Debugf("[%v] Process job exit with [%v] return code at round number [%v]",
						st.GetNameWithSuffix(),
						code.String(),
						retries)
					shouldRetry = false
				case wokerstatus.Drop:
					s.ll.S.Debugf("[%v] Process job exit with [%v] return code at round number [%v]",
						st.GetNameWithSuffix(),
						code.String(),
						retries)

					shouldRetry = false
				case wokerstatus.Retry:
					shouldRetry = true
				}
				s.ll.S.Debugf("[%v] Process exit with code [%v] at number [%v]",
					st.GetNameWithSuffix(),
					code.String(),
					retries,
				)
			}
			if retries == localMaxTry && shouldRetry {
				s.ll.S.Debugf("[%v] Process exit with error code at number [%v]. Give up",
					st.GetNameWithSuffix(),
					retries,
				)
			}

			s.ll.S.Debugf("[%v] Job finished", st.GetNameWithSuffix())
			st.After()
			jobCfg.finished = true
		})
		if err != nil {
			s.ll.S.Debugf("[%v] Cannot schedule job due to error [%v]", st.GetNameWithSuffix(), err)
			jobCfg.finished = true
			jobCfg.status = ScheduleFailed // ready to remove from job list
		} else {
			jobCfg.id = id
		}
	}

	s.c.Start()

	tick := time.NewTicker(1 * time.Second)

	defer s.c.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
			s.shouldTerminate()
		}
	}
}

func (s *Scheduler) shouldTerminate() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.ScheduleJobs) == 0 {
		return
	}

	var removedItems []int
	for idx, v := range s.ScheduleJobs {
		if v.status == ScheduleFailed {
			removedItems = append(removedItems, idx)
			continue
		}
		switch v.scheduleType {
		case OneTimeMode:
			if v.finished {
				removedItems = append(removedItems, idx)
			}
		}
	}

	for _, idx := range removedItems {
		sj := s.ScheduleJobs[idx]
		s.c.Remove(sj.id)
	}
	if len(removedItems) > 0 {
		s.ll.S.Infof("Removed %v jobs", len(removedItems))
	}
}
