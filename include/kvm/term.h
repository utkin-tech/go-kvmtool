#ifndef KVM__TERM_H
#define KVM__TERM_H

#include "kvm/kvm.h"

#include <sys/uio.h>
#include <stdbool.h>

#define CONSOLE_8250	1
#define CONSOLE_VIRTIO	2
#define CONSOLE_HV	3

#define TERM_MAX_DEVS	1

int term_putc(char *addr, int cnt, int term);
int term_getc(struct kvm *kvm, int term);

bool term_readable(int term);

#endif /* KVM__TERM_H */
