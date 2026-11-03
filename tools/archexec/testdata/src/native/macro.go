package native

/*
#include <stdlib.h>
#define CALL(a,b) a##b
static void bypass_macro(void) { CALL(sys,tem)("true"); }
*/
import "C"
