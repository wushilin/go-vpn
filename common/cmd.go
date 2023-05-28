package common

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

func cmd(args ...string) error {
	ipcmd := "/usr/sbin/ip"
	log.Println(ipcmd, strings.Join(args, " "))
	cmd := exec.Command(ipcmd, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to run: %v\n", err)
		return err
	}
	return nil
}

func AddRoute(next string) bool {
	return cmd("route", "add", next, "dev", "TUN17") == nil
}

func BringUpLink() bool {
	return cmd("link", "set", "dev", "TUN17", "up") == nil
}

func SetIPAddress(laddr string) bool {
	return cmd("addr", "add", laddr, "dev", "TUN17") == nil
}
