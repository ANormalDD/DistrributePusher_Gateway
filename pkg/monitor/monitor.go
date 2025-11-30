package monitor

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var monitorCtx context.Context
var monitorCancel context.CancelFunc

type task struct {
	sTime   int64
	lTime   int64
	success bool
}

type Monitor struct {
	name           string
	tasks          []task
	count          int
	headindex      int
	tailindex      int
	maxLen         int
	maxTaskTime    int64
	windowdur      int64
	totalTimeCount int64
	successCount   int64
	rwmu           sync.RWMutex // to protect concurrent access
	insertChan     chan *task
}

func NewMonitor(name string, maxLen int, maxTaskTime int64, windowdur int64) *Monitor {
	m := &Monitor{
		name:           name,
		tasks:          make([]task, maxLen),
		maxLen:         maxLen,
		maxTaskTime:    maxTaskTime,
		windowdur:      windowdur,
		headindex:      0,
		tailindex:      0,
		totalTimeCount: 0,
		successCount:   0,
		insertChan:     make(chan *task, maxLen),
	}
	// auto-register new monitor for exporters
	registerMonitor(m)
	return m
}

func NewTask() *task {
	return &task{
		sTime:   time.Now().UnixMilli(),
		lTime:   0,
		success: false,
	}
}

func (m *Monitor) CompleteTask(t *task, success bool) {
	t.lTime = time.Now().UnixMilli()
	t.success = success
	m.insertChan <- t
}
func (m *Monitor) GetStats() (avgTime float64, successRate float64, count int) {
	m.rwmu.RLock()
	defer m.rwmu.RUnlock()
	if m.count == 0 {
		return 0, 0, 0
	}
	avgTime = float64(m.totalTimeCount) / float64(m.count)
	successRate = float64(m.successCount) / float64(m.count)
	count = m.count
	return
}
func (m *Monitor) Run() {
	go func() {
		// ensure package monitor context is not nil to avoid nil deref
		if monitorCtx == nil {
			monitorCtx, monitorCancel = context.WithCancel(context.Background())
			zap.L().Warn("monitor: package context was not initialized; created default background context")
		}

		for {
			select {
			case <-monitorCtx.Done():
				zap.L().Info("Monitor " + m.name + " received shutdown signal, exiting")
				return
			case t := <-m.insertChan:
				m.rwmu.Lock()
				// remove old tasks outside window
				now := time.Now().UnixMilli()
				for m.headindex != m.tailindex {
					oldTask := &m.tasks[m.headindex]
					if oldTask.lTime == 0 || now-oldTask.lTime < m.windowdur {
						break
					}
					m.headindex = (m.headindex + 1) % m.maxLen
					if m.count > 0 {
						m.count--
					}
					m.totalTimeCount -= (oldTask.lTime - oldTask.sTime)
					if oldTask.success {
						if m.successCount > 0 {
							m.successCount--
						}
					}
				}

				// if buffer is full, overwrite the oldest entry (advance head)
				if m.count == m.maxLen {
					oldest := &m.tasks[m.headindex]
					m.totalTimeCount -= (oldest.lTime - oldest.sTime)
					if oldest.success {
						if m.successCount > 0 {
							m.successCount--
						}
					}
					m.headindex = (m.headindex + 1) % m.maxLen
					if m.count > 0 {
						m.count--
					}
				}

				// insert new task
				m.tasks[m.tailindex] = *t
				m.tailindex = (m.tailindex + 1) % m.maxLen
				m.count++
				m.totalTimeCount += (t.lTime - t.sTime)
				if t.success {
					m.successCount++
				}
				m.rwmu.Unlock()
			}
		}
	}()
}

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("Monitor shutdown signal received, canceling monitor context")
	if monitorCancel != nil {
		monitorCancel()
	}
	// other goroutines should observe monitorCtx.Done() and exit.
	zap.L().Info("Monitor shutdown initiated; background listeners will exit")
}

func InitMonitor() {
	monitorCtx, monitorCancel = context.WithCancel(context.Background())
	go waitForShutdown()
}
