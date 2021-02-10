package main

import (
	"math/rand"
	"time"
)

func sleepMilli(min int) {
	time.Sleep(time.Millisecond * time.Duration(min+rand.Intn(100)))
}
