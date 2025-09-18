package discord

import (
	"sync"
	"time"
)

type userLimiter struct {
	mu   sync.Mutex
	next map[string]time.Time
	win  time.Duration
}

func newUserLimiter(window time.Duration) *userLimiter {
	return &userLimiter{next: map[string]time.Time{}, win: window}
}

func (l *userLimiter) Allow(userID string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if until, ok := l.next[userID]; ok && now.Before(until) {
		return false
	}
	l.next[userID] = now.Add(l.win)
	return true
}
