# Interfaces

## Host

```
sudo nsenter -t 18623 -n /bin/bash

ip link add name br0 type bridge
ip link set br0 up

ip link set tap0 master br0
ip link set eth0 master br0

ip link set tap0 up
ip link set eth0 up
```

## Guest

```
ip link set eth0 up
ip addr add 172.17.0.2/16 dev eth0
ip route add default via 172.17.0.1 dev eth0
echo "nameserver 8.8.8.8" > /etc/resolv.conf
```

## Debug

On guest

```
nc -l -p 6060
```

On host

```
nc 172.17.0.2 6060
```

## Host macvtap

```
sudo nsenter -t 24916 -n /bin/bash

ip link add link eth0 name macvtap0 type macvtap mode bridge
ip link set macvtap0 up
```