package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	for {
		cmd("./go-vpn", os.Args[1:]...)
		log.Println("./go-vpn crashed. Starting again in 3 seconds")
		time.Sleep(3 * time.Second)
	}
}

func cmd(c string, args ...string) error {
	log.Println("Executing", c, "arguments", strings.Join(args, ","))
	cmd := exec.Command(c, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to run: %v\n", err)
		return err
	}
	return nil
}
