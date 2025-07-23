package internal

import (
	"fmt"
	"os/exec"
)

func ExecCommand(name string, commands ...string) error {
	cmd := exec.Command(name, commands...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Print(string(output))
	return nil
}
