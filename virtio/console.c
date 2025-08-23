#include <fcntl.h>
#include <linux/virtio_blk.h>
#include <linux/virtio_console.h>
#include <linux/virtio_ring.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/uio.h>
#include <unistd.h>

#include "go_exports.h"
#include "kvm/disk-image.h"
#include "kvm/guest_compat.h"
#include "kvm/ioport.h"
#include "kvm/irq.h"
#include "kvm/kvm.h"
#include "kvm/mutex.h"
#include "kvm/pci.h"
#include "kvm/threadpool.h"
#include "kvm/util.h"
#include "kvm/virtio-console.h"
#include "kvm/virtio-pci-dev.h"
#include "kvm/virtio.h"

#define VIRTIO_CONSOLE_QUEUE_SIZE 128
#define VIRTIO_CONSOLE_NUM_QUEUES 4
#define VIRTIO_CONSOLE_RX_QUEUE 0
#define VIRTIO_CONSOLE_TX_QUEUE 1
#define VIRTIO_CONSOLE_RX_QUEUE2 2
#define VIRTIO_CONSOLE_TX_QUEUE2 3

struct con_dev {
    struct mutex mutex;

    struct virtio_device vdev;
    struct virt_queue vqs[VIRTIO_CONSOLE_NUM_QUEUES];
    struct virtio_console_config config;
    int vq_ready;

    struct thread_pool__job jobs[VIRTIO_CONSOLE_NUM_QUEUES];
};

static struct con_dev cdev = {
    .mutex = MUTEX_INITIALIZER,
    .vq_ready = 0,
};

static int compat_id = -1;

int g_term_putc_iov(struct iovec *iov, int iovcnt, int term);
int g_term_getc_iov(struct kvm *kvm, struct iovec *iov, int iovcnt, int term);

/*
 * Interrupts are injected for hvc0 only.
 */
static void virtio_console__inject_interrupt_callback(struct kvm *kvm, void *param) {
    struct iovec iov[VIRTIO_CONSOLE_QUEUE_SIZE];
    struct virt_queue *vq;
    u16 out, in;
    u16 head;
    int len;

    mutex_lock(&cdev.mutex);

    vq = param;

    if (g_term_readable(0) && virt_queue__available(vq)) {
        head = virt_queue__get_iov(vq, iov, &out, &in, kvm);
        len = g_term_getc_iov(kvm, iov, in, 0);
        virt_queue__set_used_elem(vq, head, len);
        cdev.vdev.ops->signal_vq(kvm, &cdev.vdev, vq - cdev.vqs);
    }

    mutex_unlock(&cdev.mutex);
}

void virtio_console__inject_interrupt(struct kvm *kvm) {
    // if (kvm->cfg.active_console != CONSOLE_VIRTIO)
    // 	return;

    mutex_lock(&cdev.mutex);
    if (cdev.vq_ready) {
        thread_pool__do_job(&cdev.jobs[VIRTIO_CONSOLE_RX_QUEUE]);
    }

    mutex_unlock(&cdev.mutex);
}

static void virtio_console_handle_callback(struct kvm *kvm, void *param) {
    struct iovec iov[VIRTIO_CONSOLE_QUEUE_SIZE];
    struct virt_queue *vq;
    u16 out, in;
    u16 head;
    u32 len;

    vq = param;

    /*
     * The current Linux implementation polls for the buffer
     * to be used, rather than waiting for an interrupt.
     * So there is no need to inject an interrupt for the tx path.
     */

    while (virt_queue__available(vq)) {
        head = virt_queue__get_iov(vq, iov, &out, &in, kvm);
        len = g_term_putc_iov(iov, out, 0);
        virt_queue__set_used_elem(vq, head, len);
    }
}

static void virtio_console_config__inject_interrupt_callback(struct kvm *kvm, void *param) {
    struct iovec iov[VIRTIO_CONSOLE_QUEUE_SIZE];
    struct virt_queue *vq;
    u16 out, in;
    u16 head;
    int len;

    mutex_lock(&cdev.mutex);

    vq = param;

    if (virt_queue__available(vq) && has_config_event()) {
        head = virt_queue__get_iov(vq, iov, &out, &in, kvm);

        struct virtio_console_control cpkt = get_config_event();
        len = sizeof(cpkt);

        memcpy(iov[0].iov_base, &cpkt, sizeof(cpkt));

        virt_queue__set_used_elem(vq, head, len);
        cdev.vdev.ops->signal_vq(kvm, &cdev.vdev, vq - cdev.vqs);
    }

    mutex_unlock(&cdev.mutex);
}

static void virtio_console_config_handle_callback(struct kvm *kvm, void *param) {
    struct iovec iov[VIRTIO_CONSOLE_QUEUE_SIZE];
    struct virt_queue *vq;
    u16 out, in;
    u16 head;
    u32 len;

    vq = param;

    while (virt_queue__available(vq)) {
        head = virt_queue__get_iov(vq, iov, &out, &in, kvm);

        size_t len = iov[0].iov_len;
        void *buf = iov[0].iov_base;

        struct virtio_console_control cpkt, *gcpkt;
        uint8_t *buffer;
        size_t buffer_len;

        gcpkt = buf;

        if (len < sizeof(cpkt)) {
            printf("The guest sent an invalid control packet");
            return;
        }

        cpkt.event = ioport__read16(&gcpkt->event);
        cpkt.value = ioport__read16(&gcpkt->value);

        if (cpkt.event == VIRTIO_CONSOLE_PORT_READY) {
            struct virtio_console_control cpkt;
            cpkt.id = 0;
            cpkt.event = VIRTIO_CONSOLE_CONSOLE_PORT;
            cpkt.value = 1;

            put_config_event(cpkt);
        }


        virt_queue__set_used_elem(vq, head, len);
    }
}

static u8 *get_config(struct kvm *kvm, void *dev) {
    struct con_dev *cdev = dev;

    return ((u8 *)(&cdev->config));
}

static size_t get_config_size(struct kvm *kvm, void *dev) {
    struct con_dev *cdev = dev;

    return sizeof(cdev->config);
}

static u64 get_host_features(struct kvm *kvm, void *dev) {
    return (1 << VIRTIO_F_ANY_LAYOUT) | (1 << VIRTIO_CONSOLE_F_MULTIPORT);
}

static void notify_status(struct kvm *kvm, void *dev, u32 status) {
    struct con_dev *cdev = dev;
    struct virtio_console_config *conf = &cdev->config;

    if (!(status & VIRTIO__STATUS_CONFIG)) {
        return;
    }

    conf->cols = virtio_host_to_guest_u16(cdev->vdev.endian, 80);
    conf->rows = virtio_host_to_guest_u16(cdev->vdev.endian, 24);
    conf->max_nr_ports = virtio_host_to_guest_u32(cdev->vdev.endian, 1);
}

static int init_vq(struct kvm *kvm, void *dev, u32 vq) {
    struct virt_queue *queue;

    BUG_ON(vq >= VIRTIO_CONSOLE_NUM_QUEUES);

    compat__remove_message(compat_id);

    queue = &cdev.vqs[vq];

    virtio_init_device_vq(kvm, &cdev.vdev, queue, VIRTIO_CONSOLE_QUEUE_SIZE);

    if (vq == VIRTIO_CONSOLE_TX_QUEUE) {
        thread_pool__init_job(&cdev.jobs[vq], kvm, virtio_console_handle_callback, queue);
    } else if (vq == VIRTIO_CONSOLE_RX_QUEUE) {
        thread_pool__init_job(&cdev.jobs[vq], kvm, virtio_console__inject_interrupt_callback, queue);
        /* Tell the waiting poll thread that we're ready to go */
        mutex_lock(&cdev.mutex);
        cdev.vq_ready = 1;
        mutex_unlock(&cdev.mutex);
    } else if (vq == VIRTIO_CONSOLE_TX_QUEUE2) {
        thread_pool__init_job(&cdev.jobs[vq], kvm, virtio_console_config_handle_callback, queue);
    } else if (vq == VIRTIO_CONSOLE_RX_QUEUE2) {
        thread_pool__init_job(&cdev.jobs[vq], kvm, virtio_console_config__inject_interrupt_callback, queue);

        struct virtio_console_control cpkt;
        cpkt.id = 0;
        cpkt.event = VIRTIO_CONSOLE_PORT_ADD;
        cpkt.value = 1;
        put_config_event(cpkt);
    }

    return 0;
}

static void exit_vq(struct kvm *kvm, void *dev, u32 vq) {
    if (vq == VIRTIO_CONSOLE_RX_QUEUE) {
        mutex_lock(&cdev.mutex);
        cdev.vq_ready = 0;
        mutex_unlock(&cdev.mutex);
        thread_pool__cancel_job(&cdev.jobs[vq]);
    } else if (vq == VIRTIO_CONSOLE_TX_QUEUE) {
        thread_pool__cancel_job(&cdev.jobs[vq]);
    }
}

static int notify_vq(struct kvm *kvm, void *dev, u32 vq) {
    struct con_dev *cdev = dev;

    thread_pool__do_job(&cdev->jobs[vq]);

    return 0;
}

static struct virt_queue *get_vq(struct kvm *kvm, void *dev, u32 vq) {
    struct con_dev *cdev = dev;

    return &cdev->vqs[vq];
}

static int get_size_vq(struct kvm *kvm, void *dev, u32 vq) { return VIRTIO_CONSOLE_QUEUE_SIZE; }

static int set_size_vq(struct kvm *kvm, void *dev, u32 vq, int size) {
    /* FIXME: dynamic */
    return size;
}

static unsigned int get_vq_count(struct kvm *kvm, void *dev) { return VIRTIO_CONSOLE_NUM_QUEUES; }

static struct virtio_ops con_dev_virtio_ops = {
    .get_config = get_config,
    .get_config_size = get_config_size,
    .get_host_features = get_host_features,
    .get_vq_count = get_vq_count,
    .init_vq = init_vq,
    .exit_vq = exit_vq,
    .notify_status = notify_status,
    .notify_vq = notify_vq,
    .get_vq = get_vq,
    .get_size_vq = get_size_vq,
    .set_size_vq = set_size_vq,
};

int virtio_console__init(struct kvm *kvm) {
    int r;

    // if (kvm->cfg.active_console != CONSOLE_VIRTIO)
    // 	return 0;

    r = virtio_init(kvm, &cdev, &cdev.vdev, &con_dev_virtio_ops, kvm->cfg.virtio_transport,
                    PCI_DEVICE_ID_VIRTIO_CONSOLE, VIRTIO_ID_CONSOLE, PCI_CLASS_CONSOLE);
    if (r < 0)
        return r;

    if (compat_id == -1)
        compat_id = virtio_compat_add_message("virtio-console", "CONFIG_VIRTIO_CONSOLE");

    return 0;
}
virtio_dev_init(virtio_console__init);

int virtio_console__exit(struct kvm *kvm) {
    virtio_exit(kvm, &cdev.vdev);

    return 0;
}
virtio_dev_exit(virtio_console__exit);

int g_term_putc_iov(struct iovec *iov, int iovcnt, int term) {
    size_t total_written = 0;

    for (int i = 0; i < iovcnt; i++) {
        if (iov[i].iov_len > 0 && iov[i].iov_base != NULL) {
            size_t written = g_ringbuffer_write((const char *)iov[i].iov_base, iov[i].iov_len);
            total_written += written;
        }
    }

    return (int)total_written;
}

int g_term_getc_iov(struct kvm *kvm, struct iovec *iov, int iovcnt, int term) {
    int c;

    c = g_term_getc(term);

    if (c < 0) {
        return 0;
    }

    *((char *)iov[0].iov_base) = (char)c;

    return sizeof(char);
}