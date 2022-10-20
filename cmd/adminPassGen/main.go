package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/gbmor/getwtxt-ng/common"
	"golang.org/x/term"
)

func main() {
	fmt.Printf("Password: ")
	line, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Println()
	out, err := common.HashPass(string(line))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Println(string(out))
}
