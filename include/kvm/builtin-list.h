#ifndef KVM__LIST_H
#define KVM__LIST_H

#include <kvm/util.h>

int kvm_cmd_list(int argc, const char **argv, const char *prefix);
int get_vmstate(int sock);

#endif
