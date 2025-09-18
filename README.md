# kvmtool

## Preare

```bash
sudo apt-get install build-essential make zlib1g-dev socat
sudo usermod -aG kvm $USER
newgrp kvm
```

## Build

```bash
make go-build
```

## Run

```bash
./go-kvmtool -k tmp/bzImage2
```

## Connect 

```bash
socat -,raw,echo=0,escape=0x18 UNIX-CONNECT:/tmp/gkvm/term1
```

Exit keys `Ctrl + X`

```
curl -X POST --data '{"id":1}' 127.0.0.1:8080/addPort
```

## Initrd

```
mkdir -p initrd/{bin,dev,proc,sys}
cp guest/init initrd/
chmod +x initrd/init

wget https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox
cp busybox initrd/bin/sh
chmod +x initrd/bin/sh

cd initrd
find . -print0 | cpio --null -ov --format=newc | gzip -9 > ../initrd.img
```

## Create rootfs

```
mkdir tmp/rootfs
docker export $(docker create busybox) | tar -C tmp/rootfs -xvf -

mkdir tmp/rootfs/virt
cp guest/init tmp/rootfs/virt/
```
