package native

/*
#include <unistd.h>
#include <sys/syscall.h>
#include <stdlib.h>
#include <wordexp.h>
#define CALL(a,b) a##b
static void bypass(void) {
  syscall(SYS_execve, 0, 0, 0);
}
*/
import "C"
