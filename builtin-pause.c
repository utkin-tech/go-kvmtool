#include <kvm/util.h>
#include <kvm/builtin-pause.h>
#include <kvm/builtin-list.h>
#include <kvm/kvm.h>
#include <kvm/parse-options.h>
#include <kvm/kvm-ipc.h>

#include <stdio.h>
#include <string.h>
#include <signal.h>

static bool all;
static const char *instance_name;

static int do_pause(const char *name, int sock)
{
	int r;
	int vmstate;

	vmstate = get_vmstate(sock);
	if (vmstate < 0)
		return vmstate;
	if (vmstate == KVM_VMSTATE_PAUSED) {
		printf("Guest %s is already paused.\n", name);
		return 0;
	}

	r = kvm_ipc__send(sock, KVM_IPC_PAUSE);
	if (r)
		return r;

	printf("Guest %s paused\n", name);

	return 0;
}

int kvm_cmd_pause(int argc, const char **argv, const char *prefix)
{
	int instance;
	int r;

	if (all)
		return kvm__enumerate_instances(do_pause);

	if (instance_name == NULL)
		return 1; // TODO: panic

	instance = kvm__get_sock_by_instance(instance_name);

	if (instance <= 0)
		die("Failed locating instance");

	r = do_pause(instance_name, instance);

	close(instance);

	return r;
}
