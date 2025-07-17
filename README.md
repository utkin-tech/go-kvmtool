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
socat -,raw,echo=0,escape=0x18 UNIX-CONNECT:/tmp/example.sock
```

Exit keys `Ctrl + X`
