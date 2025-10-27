//go:build !windows

package main

import "wireflow/cmd"

func main() {
	cmd.Execute()
}
