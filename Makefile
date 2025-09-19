#
# Define WERROR=0 to disable -Werror.
#

ifeq ($(strip $(V)),)
	ifeq ($(findstring s,$(filter-out --%,$(firstword $(MAKEFLAGS)))),)
		E = @echo
	else
		E = @\#
	endif
	Q = @
else
	E = @\#
	Q =
endif
export E Q

CC	:= $(CROSS_COMPILE)gcc
CFLAGS	:=
LD	:= $(CROSS_COMPILE)ld
LDFLAGS	:=
OBJCOPY	:= $(CROSS_COMPILE)objcopy

# Translate uname -m into ARCH string
ARCH ?= $(shell uname -m | sed -e s/i.86/i386/ -e s/ppc.*/powerpc/ \
	  -e s/armv.*/arm/ -e s/aarch64.*/arm64/ -e s/mips64/mips/ \
	  -e s/riscv64/riscv/ -e s/riscv32/riscv/)

ifeq ($(ARCH),i386)
	override ARCH = x86
	DEFINES      += -DCONFIG_X86_32
endif
ifeq ($(ARCH),x86_64)
	override ARCH = x86
	DEFINES      += -DCONFIG_X86_64
endif

### Arch-specific stuff

#x86
ifeq ($(ARCH),x86)
	DEFINES += -DCONFIG_X86
	ARCH_INCLUDE := x86/include
endif
# POWER/ppc:  Actually only support ppc64 currently.
ifeq ($(ARCH), powerpc)
	DEFINES += -DCONFIG_PPC
	ARCH_INCLUDE := powerpc/include
endif

# ARM64
ifeq ($(ARCH), arm64)
	DEFINES		+= -DCONFIG_ARM64
	ARCH_INCLUDE	:= arm64/include
endif

ifeq ($(ARCH),mips)
	DEFINES		+= -DCONFIG_MIPS
	ARCH_INCLUDE	:= mips/include
endif

# RISC-V (RV32 and RV64)
ifeq ($(ARCH),riscv)
	DEFINES		+= -DCONFIG_RISCV
	ARCH_INCLUDE	:= riscv/include
	ifeq ($(RISCV_XLEN),32)
		CFLAGS	+= -mabi=ilp32d -march=rv32gc
	endif
	ifeq ($(RISCV_XLEN),64)
		CFLAGS	+= -mabi=lp64d -march=rv64gc
	endif
endif
###

ifeq (,$(ARCH_INCLUDE))
        $(error This architecture ($(ARCH)) is not supported in kvmtool)
endif

DEFINES	+= -D_FILE_OFFSET_BITS=64
DEFINES	+= -D_GNU_SOURCE
DEFINES	+= -DKVMTOOLS_VERSION='"$(KVMTOOLS_VERSION)"'
DEFINES	+= -DBUILD_ARCH='"$(ARCH)"'

KVM_INCLUDE := include
CFLAGS	+= $(CPPFLAGS) $(DEFINES) -I$(KVM_INCLUDE) -I$(ARCH_INCLUDE) -O2 -fno-strict-aliasing -g

WARNINGS += -Wall
WARNINGS += -Wformat=2
WARNINGS += -Winit-self
WARNINGS += -Wmissing-declarations
WARNINGS += -Wmissing-prototypes
WARNINGS += -Wnested-externs
WARNINGS += -Wno-system-headers
WARNINGS += -Wold-style-definition
WARNINGS += -Wredundant-decls
WARNINGS += -Wsign-compare
WARNINGS += -Wstrict-prototypes
WARNINGS += -Wundef
WARNINGS += -Wvolatile-register-var
WARNINGS += -Wwrite-strings
WARNINGS += -Wno-format-nonliteral

CFLAGS	+= $(WARNINGS)

ifneq ($(WERROR),0)
	CFLAGS += -Werror
endif

#
# BIOS assembly weirdness
#
BIOS_CFLAGS += -m32
BIOS_CFLAGS += -march=i386
BIOS_CFLAGS += -mregparm=3

BIOS_CFLAGS += -fno-stack-protector
BIOS_CFLAGS += -fno-pic

x86/bios.o: x86/bios/bios.bin x86/bios/bios-rom.h

x86/bios/bios.bin.elf: x86/bios/entry.S x86/bios/e820.c x86/bios/int10.c x86/bios/int15.c x86/bios/rom.ld.S
	$(E) "  CC       x86/bios/memcpy.o"
	$(Q) $(CC) -include code16gcc.h $(CFLAGS) $(BIOS_CFLAGS) -c x86/bios/memcpy.c -o x86/bios/memcpy.o
	$(E) "  CC       x86/bios/e820.o"
	$(Q) $(CC) -include code16gcc.h $(CFLAGS) $(BIOS_CFLAGS) -c x86/bios/e820.c -o x86/bios/e820.o
	$(E) "  CC       x86/bios/int10.o"
	$(Q) $(CC) -include code16gcc.h $(CFLAGS) $(BIOS_CFLAGS) -c x86/bios/int10.c -o x86/bios/int10.o
	$(E) "  CC       x86/bios/int15.o"
	$(Q) $(CC) -include code16gcc.h $(CFLAGS) $(BIOS_CFLAGS) -c x86/bios/int15.c -o x86/bios/int15.o
	$(E) "  CC       x86/bios/entry.o"
	$(Q) $(CC) $(CFLAGS) $(BIOS_CFLAGS) -c x86/bios/entry.S -o x86/bios/entry.o
	$(E) "  LD      " $@
	$(Q) $(LD) -T x86/bios/rom.ld.S -o x86/bios/bios.bin.elf x86/bios/memcpy.o x86/bios/entry.o x86/bios/e820.o x86/bios/int10.o x86/bios/int15.o

x86/bios/bios.bin: x86/bios/bios.bin.elf
	$(E) "  OBJCOPY " $@
	$(Q) $(OBJCOPY) -O binary -j .text x86/bios/bios.bin.elf x86/bios/bios.bin

x86/bios/bios-rom.o: x86/bios/bios-rom.S x86/bios/bios.bin x86/bios/bios-rom.h
	$(E) "  CC      " $@
	$(Q) $(CC) -c $(CFLAGS) x86/bios/bios-rom.S -o x86/bios/bios-rom.o

x86/bios/bios-rom.h: x86/bios/bios.bin.elf
	$(E) "  NM      " $@
	$(Q) cd x86/bios && sh gen-offsets.sh > bios-rom.h && cd ..

clean:
	$(E) "  CLEAN"
	$(Q) rm -f x86/bios/*.bin
	$(Q) rm -f x86/bios/*.elf
	$(Q) rm -f x86/bios/*.o
	$(Q) rm -f x86/bios/bios-rom.h
	$(Q) rm -f guest/init
	$(Q) rm -rf tmp/rootfs
.PHONY: clean

guest/init:
	cd guest && CGO_ENABLED=0 go build -o init -ldflags="-s -w"

tmp/rootfs:
	mkdir -p tmp/rootfs
	docker export $$(docker create busybox) | tar -C tmp/rootfs -xvf -

tmp/rootfs/virt: tmp/rootfs guest/init
	mkdir -p tmp/rootfs/virt
	cp guest/init tmp/rootfs/virt/

image: tmp/rootfs/virt
	@echo "Root filesystem prepared in tmp/rootfs"

go-build: x86/bios/bios-rom.h x86/bios/bios.bin
	go build
