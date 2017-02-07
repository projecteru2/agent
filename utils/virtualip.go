package utils

import (
	"bytes"
	"fmt"
	"os/exec"
)

func execCmd(name string, args []string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func SetVip(vip, inter string) error {
	temp := fmt.Sprintf("%s/32", vip)
	ipArgs := []string{"addr", "add", temp, "dev", inter}
	_, err := execCmd("ip", ipArgs)
	if err != nil {
		return err
	}
	arpingArgs := []string{"-c3", "-U", "-I", inter, vip}
	_, err = execCmd("arping", arpingArgs)
	return err
}

func DelVip(vip, inter string) error {
	temp := fmt.Sprintf("%s/32", vip)
	args := []string{"addr", "del", temp, "dev", inter}
	_, err := execCmd("ip", args)
	return err
}

func CheckVip(inter string) (string, error) {
	ipArgs := []string{"-o", "-4", "addr", "show", inter}
	return execCmd("ip", ipArgs)
}
