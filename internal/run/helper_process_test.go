package run

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("AGENTLAB_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "clean":
		time.Sleep(30 * time.Millisecond)
		fmt.Println(`{"type":"message","text":"working"}`)
		fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success","value":{"ok":true}}`)
	case "duplicate":
		time.Sleep(30 * time.Millisecond)
		fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success"}`)
		fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success"}`)
	case "missing":
		time.Sleep(30 * time.Millisecond)
		fmt.Println(`{"type":"message"}`)
	case "nonzero":
		time.Sleep(30 * time.Millisecond)
		os.Exit(7)
	case "silent":
		time.Sleep(5 * time.Second)
	case "continuous":
		for index := range 4 {
			fmt.Printf("{\"type\":\"message\",\"text\":\"step-%d\"}\n", index)
			time.Sleep(40 * time.Millisecond)
		}
		fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success","value":{"ok":true}}`)
	case "environment":
		payload, _ := json.Marshal(map[string]string{
			"public": os.Getenv("PUBLIC_VALUE"), "secret": os.Getenv("SECRET_VALUE"), "parent": os.Getenv("PARENT_SECRET"),
		})
		fmt.Printf("{\"type\":\"message\",\"text\":%s}\n", payload)
		fmt.Println(`{"type":"result","contract":"agentlab.worker-result.v1","outcome":"success","value":{"ok":true}}`)
	case "group":
		signal.Ignore(syscall.SIGTERM)
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "group-child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		fmt.Printf("child_pid=%d\n", child.Process.Pid)
		select {}
	case "group-child":
		signal.Ignore(syscall.SIGTERM)
		select {}
	}
	os.Exit(0)
}
