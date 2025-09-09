#ifndef GO_EXPORTS_H
#define GO_EXPORTS_H

#include <stdbool.h>

bool g_ringbuffer_write(const char *, int, int);
int g_term_getc(int);
bool g_term_readable(int);

bool has_config_event(void);
struct virtio_console_control get_config_event(void);
void put_config_event(struct virtio_console_control);

#endif