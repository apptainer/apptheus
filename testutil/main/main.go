package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	conn, err := net.Dial("unix", "/run/apptheus/gateway.sock")
	if err != nil {
		fmt.Printf("connection error: %v", err)
		os.Exit(-1)
	}

	defer conn.Close()

	for {
		sum := uint64(0)
		for i := 0; i < 10000000; i++ {
			sum = sum + uint64(i)
		}
		uid := os.Getuid()
		pid := os.Getpid()
		fmt.Printf("calculation completed, my uid: %d, my pid: %d\n", uid, pid)
		time.Sleep(time.Second * 5)
	}
}
