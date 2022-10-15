// +build windows

package getpass

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var getStdHandle, getConsoleMode, setConsoleMode *syscall.LazyProc

func init() {
	lib := syscall.NewLazyDLL("Kernel32.dll")
	getStdHandle = lib.NewProc("GetStdHandle")
	getConsoleMode = lib.NewProc("GetConsoleMode")
	setConsoleMode = lib.NewProc("SetConsoleMode")
}

func Prompt(msg string) string {
	fmt.Print(msg + ": ")
	handle, _, _ := getStdHandle.Call(uintptr(^uint(10) + 1))
	var mode uint
	getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	newMode := mode & ^uint(4)
	setConsoleMode.Call(handle, uintptr(newMode))
	passwd, err := bufio.NewReader(os.Stdin).ReadString('\n')
	setConsoleMode.Call(handle, uintptr(mode))
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(passwd)
}
