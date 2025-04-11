package main

import (
	"time"
)

func StartScheduler() {
	go func() {
		for {
			AutoCleanupExpiredRoleBindings()
			time.Sleep(1 * time.Minute)
		}
	}()
}
