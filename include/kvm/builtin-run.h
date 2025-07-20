#ifndef __KVM_RUN_H__
#define __KVM_RUN_H__

#include <kvm/util.h>

int kvm_cmd_run(int fd_in, int fd_out, const char *kernel_filename);

void kvm_run_set_wrapper_sandbox(void);

#endif
