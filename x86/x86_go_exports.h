#ifndef X86_GO_EXPORTS_H
#define X86_GO_EXPORTS_H

#include <sys/types.h>

int get_initrd_size();
ssize_t read_in_full_initrd(void*);

#endif