package tools

import (
	"context"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	mrand "math/rand"

	"golang.org/x/time/rate"
)

// Contains 一个字符串是否存在于一个字符串数组中
func Contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// Go 捕获panic并输出错误
func Go(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic ❌",
					"Goroutine", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				os.Exit(1)
			}
		}()
		fn()
	}()
}

type JitterLimiter struct {
	limiter   *rate.Limiter
	minJitter time.Duration
	maxJitter time.Duration
}

func NewJitterLimiter(perMinute int, minJitter, maxJitter time.Duration) *JitterLimiter {
	r := rate.Every(time.Minute / time.Duration(perMinute))
	return &JitterLimiter{
		limiter:   rate.NewLimiter(r, perMinute),
		minJitter: minJitter,
		maxJitter: maxJitter,
	}
}

func (l *JitterLimiter) Wait(ctx context.Context) error {
	// 1️⃣ 先随机 sleep
	jitter := l.minJitter
	if l.maxJitter > l.minJitter {

		jitter += time.Duration(mrand.Int63n(
			int64(l.maxJitter - l.minJitter),
		))
	}
	time.Sleep(jitter)

	// 2️⃣ 再等待限速器放行
	return l.limiter.Wait(ctx)
}
