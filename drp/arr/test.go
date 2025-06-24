package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now().UnixMilli()
	time.Sleep(10 * time.Second)

	fmt.Println("时间差：", time.Since(time.UnixMilli(now)).Seconds())
}
