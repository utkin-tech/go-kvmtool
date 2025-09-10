#include <stdio.h>
#include <string.h>
#include <signal.h>

#include <kvm/util.h>
#include <kvm/builtin-balloon.h>
#include <kvm/parse-options.h>
#include <kvm/kvm.h>
#include <kvm/kvm-ipc.h>

static const char *instance_name;
static u64 inflate;
static u64 deflate;

int kvm_cmd_balloon(int argc, const char **argv, const char *prefix)
{
	int instance;
	int r;
	int amount;

	if (inflate == 0 && deflate == 0)
		return 1; // TODO: panic

	if (instance_name == NULL)
		return 1; // TODO: panic

	instance = kvm__get_sock_by_instance(instance_name);

	if (instance <= 0)
		die("Failed locating instance");

	if (inflate)
		amount = inflate;
	else if (deflate)
		amount = -deflate;
	else
		return 1; // TODO: panic

	r = kvm_ipc__send_msg(instance, KVM_IPC_BALLOON,
			sizeof(amount), (u8 *)&amount);

	close(instance);

	if (r < 0)
		return -1;

	return 0;
}
