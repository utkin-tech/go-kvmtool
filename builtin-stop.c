#include <kvm/util.h>
#include <kvm/builtin-stop.h>
#include <kvm/kvm.h>
#include <kvm/parse-options.h>
#include <kvm/kvm-ipc.h>

#include <stdio.h>
#include <string.h>
#include <signal.h>

static bool all;
static const char *instance_name;

static int do_stop(const char *name, int sock)
{
	return kvm_ipc__send(sock, KVM_IPC_STOP);
}

int kvm_cmd_stop(int argc, const char **argv, const char *prefix)
{
	int instance;
	int r;

	if (all)
		return kvm__enumerate_instances(do_stop);

	if (instance_name == NULL)
		return 1; // TODO: panic

	instance = kvm__get_sock_by_instance(instance_name);

	if (instance <= 0)
		die("Failed locating instance");

	r = do_stop(instance_name, instance);

	close(instance);

	return r;
}
