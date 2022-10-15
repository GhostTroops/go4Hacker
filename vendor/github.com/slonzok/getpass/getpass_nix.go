// +build darwin linux

package getpass

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

/*
#include <stdio.h>
#include <termios.h>
struct termios disable_echo() {
  struct termios of, nf;
  tcgetattr(fileno(stdin), &of);
  nf = of;
  nf.c_lflag &= ~ECHO;
  nf.c_lflag |= ECHONL;
  if (tcsetattr(fileno(stdin), TCSANOW, &nf) != 0) {
    perror("tcsetattr");
  }
  return of;
}

void restore_echo(struct termios f) {
  if (tcsetattr(fileno(stdin), TCSANOW, &f) != 0) {
    perror("tcsetattr");
  }
}
*/
import "C"

func Prompt(msg string) string {
	fmt.Printf("%s: ", msg)
	oldFlags := C.disable_echo()
	passwd, err := bufio.NewReader(os.Stdin).ReadString('\n')
	C.restore_echo(oldFlags)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(passwd)
}

/*import (
	"sync"
	"unsafe"
)

/*
#include <stdlib.h>
#include <unistd.h>
*/
/*import "C"

var mtx sync.Mutex

func Prompt(msg string) string {
	s := C.CString(msg + ": ")
	defer C.free(unsafe.Pointer(s))
	mtx.Lock()
	defer mtx.Unlock()
	passwd := C.getpass(s)
	return C.GoString(passwd)
}
*/
