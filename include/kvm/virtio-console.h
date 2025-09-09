#ifndef KVM__CONSOLE_VIRTIO_H
#define KVM__CONSOLE_VIRTIO_H

struct kvm;

int virtio_console__init(struct kvm *kvm);
void virtio_console__inject_interrupt(struct kvm *kvm);
int virtio_console__exit(struct kvm *kvm);

void virtio_console__inject_interrupt_vq(struct kvm *kvm, int vq);
void virtio_console_config__inject_interrupt();
#endif /* KVM__CONSOLE_VIRTIO_H */
