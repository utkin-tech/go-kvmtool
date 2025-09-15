#include "kvm/builtin-run.h"

#include "kvm/8250-serial.h"
#include "kvm/barrier.h"
#include "kvm/builtin-debug.h"
#include "kvm/disk-image.h"
#include "kvm/framebuffer.h"
#include "kvm/guest_compat.h"
#include "kvm/i8042.h"
#include "kvm/ioeventfd.h"
#include "kvm/ioport.h"
#include "kvm/irq.h"
#include "kvm/kvm-cpu.h"
#include "kvm/kvm-ipc.h"
#include "kvm/kvm.h"
#include "kvm/mutex.h"
#include "kvm/pci.h"
#include "kvm/rtc.h"
#include "kvm/sdl.h"
#include "kvm/strbuf.h"
#include "kvm/symbol.h"
#include "kvm/term.h"
#include "kvm/threadpool.h"
#include "kvm/util.h"
#include "kvm/vesa.h"
#include "kvm/virtio-9p.h"
#include "kvm/virtio-balloon.h"
#include "kvm/virtio-blk.h"
#include "kvm/virtio-console.h"
#include "kvm/virtio-net.h"
#include "kvm/virtio-rng.h"
#include "kvm/virtio-scsi.h"
#include "kvm/vnc.h"

#include <ctype.h>
#include <linux/err.h>
#include <linux/sizes.h>
#include <linux/types.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/utsname.h>
#include <unistd.h>

#define KB_SHIFT (10)
#define MB_SHIFT (20)
#define GB_SHIFT (30)
#define TB_SHIFT (40)
#define PB_SHIFT (50)

__thread struct kvm_cpu *current_kvm_cpu;

int loglevel = LOGLEVEL_INFO;

static void *kvm_cpu_thread(void *arg) {
    char name[16];

    current_kvm_cpu = arg;

    sprintf(name, "kvm-vcpu-%lu", current_kvm_cpu->cpu_id);
    kvm__set_thread_name(name);

    if (kvm_cpu__start(current_kvm_cpu))
        goto panic_kvm;

    return ERR_PTR(0);

panic_kvm:
    pr_err("KVM exit reason: %u (\"%s\")", current_kvm_cpu->kvm_run->exit_reason,
           kvm_exit_reasons[current_kvm_cpu->kvm_run->exit_reason]);

    if (current_kvm_cpu->kvm_run->exit_reason == KVM_EXIT_UNKNOWN) {
        pr_err("KVM exit code: %llu", (unsigned long long)current_kvm_cpu->kvm_run->hw.hardware_exit_reason);
    }

    kvm_cpu__set_debug_fd(STDOUT_FILENO);
    kvm_cpu__show_registers(current_kvm_cpu);
    kvm_cpu__show_code(current_kvm_cpu);
    kvm_cpu__show_page_tables(current_kvm_cpu);

    return ERR_PTR(1);
}

static char kernel[PATH_MAX];

static long host_page_size(void) {
    long page_size = sysconf(_SC_PAGE_SIZE);

    if (page_size < 0) {
        pr_warning("sysconf(_SC_PAGE_SIZE) failed");
        return 0;
    }

    return page_size;
}

static long host_ram_nrpages(void) {
    long nr_pages = sysconf(_SC_PHYS_PAGES);

    if (nr_pages < 0) {
        pr_warning("sysconf(_SC_PHYS_PAGES) failed");
        return 0;
    }

    return nr_pages;
}

static u64 host_ram_size(void) {
    long page_size = host_page_size();
    long nr_pages = host_ram_nrpages();

    return (u64)nr_pages * page_size;
}

/*
 * If user didn't specify how much memory it wants to allocate for the guest,
 * avoid filling the whole host RAM.
 */
#define RAM_SIZE_RATIO 0.8

static u64 get_ram_size(int nr_cpus) {
    long nr_pages_available = host_ram_nrpages() * RAM_SIZE_RATIO;
    u64 ram_size = (u64)SZ_64M * (nr_cpus + 3);
    u64 available = MIN_RAM_SIZE;

    if (nr_pages_available) {
        available = nr_pages_available * host_page_size();
    }

    if (ram_size > available) {
        ram_size = available;
    }

    return ram_size;
}

static void kvm_run_set_real_cmdline(struct kvm *kvm) {
    static char real_cmdline[2048];
    bool video;

    video = kvm->cfg.vnc || kvm->cfg.sdl || kvm->cfg.gtk;

    memset(real_cmdline, 0, sizeof(real_cmdline));
    kvm__arch_set_cmdline(real_cmdline, video);

    if (video) {
        strcat(real_cmdline, " console=tty0");
    } else {
        switch (kvm->cfg.active_console) {
        case CONSOLE_HV:
            /* Fallthrough */
        case CONSOLE_VIRTIO:
            strcat(real_cmdline, " console=hvc0");
            break;
        case CONSOLE_8250:
            strcat(real_cmdline, " console=ttyS0");
            break;
        }
    }

    if (kvm->cfg.using_rootfs) {
        strcat(real_cmdline, " rw rootflags=trans=virtio,version=9p2000.L,cache=loose rootfstype=9p");
        if (kvm->cfg.custom_rootfs) {
            strcat(real_cmdline, " init=/virt/init");
            if (!kvm->cfg.no_dhcp) {
                strcat(real_cmdline, "  ip=dhcp");
            }
        }
    } else if (!kvm->cfg.kernel_cmdline || !strstr(kvm->cfg.kernel_cmdline, "root=")) {
        strlcat(real_cmdline, " root=/dev/vda rw ", sizeof(real_cmdline));
    }

    if (kvm->cfg.kernel_cmdline) {
        strcat(real_cmdline, " ");
        strlcat(real_cmdline, kvm->cfg.kernel_cmdline, sizeof(real_cmdline));
    }

    kvm->cfg.real_cmdline = real_cmdline;
}

static void kvm_run_validate_cfg(struct kvm *kvm) {
    u64 available_ram;

    if (kvm->cfg.kernel_filename && kvm->cfg.firmware_filename) {
        die("Only one of --kernel or --firmware can be specified");
    }

    if ((kvm->cfg.vnc && (kvm->cfg.sdl || kvm->cfg.gtk)) || (kvm->cfg.sdl && kvm->cfg.gtk)) {
        die("Only one of --vnc, --sdl or --gtk can be specified");
    }

    if (kvm->cfg.firmware_filename && kvm->cfg.initrd_filename) {
        pr_warning("Ignoring initrd file when loading a firmware image");
    }

    if (kvm->cfg.ram_size) {
        available_ram = host_ram_size();
        if (available_ram && kvm->cfg.ram_size > available_ram) {
            pr_warning("Guest memory size %lluMB exceeds host physical RAM size %lluMB",
                       (unsigned long long)kvm->cfg.ram_size >> MB_SHIFT,
                       (unsigned long long)available_ram >> MB_SHIFT);
        }
    }

    kvm__arch_validate_cfg(kvm);
}

static struct kvm *kvm_cmd_run_init(const char *kernel_filename) {
    static char default_name[20];
    unsigned int nr_online_cpus;
    struct kvm *kvm = kvm__new();

    if (IS_ERR(kvm)) {
        return kvm;
    }

    nr_online_cpus = sysconf(_SC_NPROCESSORS_ONLN);
    kvm->cfg.custom_rootfs_name = "default";
    /*
     * An architecture can allow the user to set the RAM base address to
     * zero. Initialize the address before parsing the command line
     * arguments, otherwise it will be impossible to distinguish between the
     * user setting the base address to zero or letting it unset and using
     * the default value.
     */
    kvm->cfg.ram_addr = kvm__arch_default_ram_address();

    kvm->cfg.kernel_filename = kernel_filename;
    kvm->cfg.console = "virtio";

    kvm_run_validate_cfg(kvm);

    if (kvm->cfg.nrcpus == 0) {
        kvm->cfg.nrcpus = nr_online_cpus;
    }

    if (!kvm->cfg.ram_size) {
        kvm->cfg.ram_size = get_ram_size(kvm->cfg.nrcpus);
    }

    if (!kvm->cfg.dev) {
        kvm->cfg.dev = DEFAULT_KVM_DEV;
    }

    if (!kvm->cfg.console) {
        kvm->cfg.console = DEFAULT_CONSOLE;
    }

    if (!strncmp(kvm->cfg.console, "virtio", 6)) {
        kvm->cfg.active_console = CONSOLE_VIRTIO;
    } else if (!strncmp(kvm->cfg.console, "serial", 6)) {
        kvm->cfg.active_console = CONSOLE_8250;
    } else if (!strncmp(kvm->cfg.console, "hv", 2)) {
        kvm->cfg.active_console = CONSOLE_HV;
    } else {
        pr_warning("No console!");
    }

    if (!kvm->cfg.host_ip) {
        kvm->cfg.host_ip = DEFAULT_HOST_ADDR;
    }

    if (!kvm->cfg.guest_ip) {
        kvm->cfg.guest_ip = DEFAULT_GUEST_ADDR;
    }

    if (!kvm->cfg.guest_mac) {
        kvm->cfg.guest_mac = DEFAULT_GUEST_MAC;
    }

    if (!kvm->cfg.host_mac) {
        kvm->cfg.host_mac = DEFAULT_HOST_MAC;
    }

    if (!kvm->cfg.script) {
        kvm->cfg.script = DEFAULT_SCRIPT;
    }

    if (!kvm->cfg.network) {
        kvm->cfg.network = DEFAULT_NETWORK;
    }

    if (!kvm->cfg.guest_name) {
        if (kvm->cfg.custom_rootfs) {
            kvm->cfg.guest_name = kvm->cfg.custom_rootfs_name;
        } else {
            sprintf(default_name, "guest-%u", getpid());
            kvm->cfg.guest_name = default_name;
        }
    }

    if (!kvm->cfg.nodefaults && !kvm->cfg.using_rootfs && !kvm->cfg.disk_image[0].filename &&
        !kvm->cfg.initrd_filename) {
        char tmp[PATH_MAX];

        // HERE CHANGE TO RUNC ROOTFS
        snprintf(tmp, PATH_MAX, "/home/user/go-kvmtool/tmp/rootfs");
        if (virtio_9p__register(kvm, tmp, "/dev/root") < 0) {
            die("Unable to initialize virtio 9p");
        }
        kvm->cfg.using_rootfs = kvm->cfg.custom_rootfs = 1;
    }

    if (kvm->cfg.nodefaults) {
        kvm->cfg.real_cmdline = kvm->cfg.kernel_cmdline;
    } else {
        kvm_run_set_real_cmdline(kvm);
    }

    if (kvm->cfg.kernel_filename) {
        pr_info("# %s run -k %s -m %Lu -c %d --name %s", KVM_BINARY_NAME, kvm->cfg.kernel_filename,
                (unsigned long long)kvm->cfg.ram_size >> MB_SHIFT, kvm->cfg.nrcpus, kvm->cfg.guest_name);
    } else if (kvm->cfg.firmware_filename) {
        pr_info("# %s run --firmware %s -m %Lu -c %d --name %s", KVM_BINARY_NAME, kvm->cfg.firmware_filename,
                (unsigned long long)kvm->cfg.ram_size >> MB_SHIFT, kvm->cfg.nrcpus, kvm->cfg.guest_name);
    }

    if (init_list__init(kvm) < 0) {
        die("Initialisation failed");
    }

    return kvm;
}

static int kvm_cmd_run_work(struct kvm *kvm) {
    void *vcpu0_ret;
    int i;

    for (i = 0; i < kvm->nrcpus; i++) {
        if (pthread_create(&kvm->cpus[i]->thread, NULL, kvm_cpu_thread, kvm->cpus[i]) != 0) {
            die("unable to create KVM VCPU thread");
        }
    }

    /* Only VCPU #0 is going to exit by itself when shutting down */
    if (pthread_join(kvm->cpus[0]->thread, &vcpu0_ret) != 0) {
        die("unable to join with vcpu 0");
    }

    return kvm_cpu__exit(kvm, PTR_ERR(vcpu0_ret));
}

static void kvm_cmd_run_exit(struct kvm *kvm, int guest_ret) {
    compat__print_all_messages();

    init_list__exit(kvm);

    if (guest_ret == 0) {
        pr_info("KVM session ended normally.");
    }
}

int kvm_cmd_run(const char *kernel_filename) {
    int ret = -EFAULT;
    struct kvm *kvm;

    kvm = kvm_cmd_run_init(kernel_filename);
    if (IS_ERR(kvm)) {
        return PTR_ERR(kvm);
    }

    ret = kvm_cmd_run_work(kvm);
    kvm_cmd_run_exit(kvm, ret);

    return ret;
}
