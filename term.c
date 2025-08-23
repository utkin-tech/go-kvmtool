#include "kvm/term.h"

#include <poll.h>
#include <pty.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <sys/uio.h>
#include <termios.h>
#include <unistd.h>
#include <utmp.h>

#include "kvm/kvm-cpu.h"
#include "kvm/kvm.h"
#include "kvm/read-write.h"
#include "kvm/util.h"

int term_getc(struct kvm *kvm, int term) { return 0; }

int term_putc(char *addr, int cnt, int term) {}

bool term_readable(int term) { return false; }
