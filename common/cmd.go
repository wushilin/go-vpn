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

func AddRoute(device, next string) bool {
	return cmd("route", "add", next, "dev", device) == nil
}

func BringUpLink(device string) bool {
	return cmd("link", "set", "dev", device, "up") == nil
}

func SetIPAddress(device, laddr string) bool {
	return cmd("addr", "add", laddr, "dev", device) == nil
}
