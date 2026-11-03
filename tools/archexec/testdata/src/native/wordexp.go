package native

/*
#include <wordexp.h>
static void bypass_wordexp(void) { wordexp_t result; wordexp("$(true)", &result, 0); }
*/
import "C"
